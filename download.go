package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var unsafeFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func slugify(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = unsafeFilenameChars.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		return "unknown"
	}
	return s
}

// saveApplicant writes metadata + every present attachment to dir.
// Returns the list of newly-written filenames and the list of skipped (already-existing) filenames.
func saveApplicant(dir string, a *applicantDetail, c *client) (written, skipped []string, err error) {
	slug := slugify(a.Candidate.FirstName + "_" + a.Candidate.LastName)
	prefix := fmt.Sprintf("%d_%s", a.ID, slug)

	metaPath := filepath.Join(dir, prefix+".json")
	if exists(metaPath) {
		skipped = append(skipped, filepath.Base(metaPath))
	} else {
		if err := writeJSON(metaPath, a.Raw); err != nil {
			return written, skipped, fmt.Errorf("write metadata: %w", err)
		}
		written = append(written, filepath.Base(metaPath))
	}

	kinds := []struct {
		name string
		att  *attachment
	}{
		{"cv", a.Candidate.Resume},
		{"cover", a.Candidate.CoverLetter},
		{"portfolio", a.Candidate.Portfolio},
	}
	for _, k := range kinds {
		if k.att == nil || k.att.URL == "" {
			continue
		}
		ext := extOf(k.att.ContentFileName)
		dest := filepath.Join(dir, fmt.Sprintf("%s_%s%s", prefix, k.name, ext))
		if exists(dest) {
			skipped = append(skipped, filepath.Base(dest))
			continue
		}
		if err := downloadFile(c, k.att.URL, dest); err != nil {
			return written, skipped, fmt.Errorf("download %s: %w", k.name, err)
		}
		written = append(written, filepath.Base(dest))
	}

	return written, skipped, nil
}

func extOf(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return ""
	}
	if len(ext) > 8 {
		return ""
	}
	return strings.ToLower(ext)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// downloadFile fetches url to dest. Tries with the Freshteam bearer token first;
// if the URL is a pre-signed external link that rejects the Authorization header
// (401/403), retries without it.
func downloadFile(c *client, url, dest string) error {
	if err := tryDownload(c, url, dest, true); err == nil {
		return nil
	} else if !isAuthRejection(err) {
		return err
	}
	return tryDownload(c, url, dest, false)
}

type authErr struct{ status int }

func (e *authErr) Error() string { return fmt.Sprintf("http %d", e.status) }

func isAuthRejection(err error) bool {
	var ae *authErr
	return errors.As(err, &ae)
}

func tryDownload(c *client, url, dest string, withAuth bool) (retErr error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	if withAuth {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &authErr{status: resp.StatusCode}
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("GET %s: %s — %s", url, resp.Status, body)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() {
		if retErr != nil {
			_ = os.Remove(tmp)
		}
	}()
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	return nil
}

