package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type client struct {
	domain string
	token  string
	http   *retryablehttp.Client
}

func newClient(domain, token string) *client {
	rc := retryablehttp.NewClient()
	rc.RetryMax = 4
	rc.RetryWaitMin = 1 * time.Second
	rc.RetryWaitMax = 30 * time.Second
	rc.Logger = nil
	return &client{domain: domain, token: token, http: rc}
}

func (c *client) do(method, path string) ([]byte, http.Header, error) {
	url := "https://" + c.domain + path
	req, err := retryablehttp.NewRequest(method, url, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, resp.Header, fmt.Errorf("GET %s: %s — %s", path, resp.Status, truncate(body, 300))
	}
	return body, resp.Header, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}

type jobPosting struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func (c *client) getJobPosting(id int) (*jobPosting, error) {
	body, _, err := c.do("GET", fmt.Sprintf("/api/job_postings/%d", id))
	if err != nil {
		return nil, err
	}
	var jp jobPosting
	if err := json.Unmarshal(body, &jp); err != nil {
		return nil, fmt.Errorf("decode posting: %w", err)
	}
	return &jp, nil
}

type applicantSummary struct {
	ID        int             `json:"id"`
	JobID     int             `json:"job_id"`
	Candidate candidateBasics `json:"candidate"`
}

type candidateBasics struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

func (a applicantSummary) displayName() string {
	name := a.Candidate.FirstName + " " + a.Candidate.LastName
	if name == " " {
		return fmt.Sprintf("applicant-%d", a.ID)
	}
	return name
}

func (c *client) listApplicants(roleID int) ([]applicantSummary, error) {
	var all []applicantSummary
	page := 1
	for {
		body, hdr, err := c.do("GET", fmt.Sprintf("/api/job_postings/%d/applicants?page=%d", roleID, page))
		if err != nil {
			return nil, err
		}
		var batch []applicantSummary
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("decode applicants page %d: %w", page, err)
		}
		all = append(all, batch...)

		totalPages := 1
		if v := hdr.Get("total-pages"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				totalPages = n
			}
		}
		if page >= totalPages || len(batch) == 0 {
			break
		}
		page++
	}
	return all, nil
}

type attachment struct {
	ContentFileName string `json:"content_file_name"`
	URL             string `json:"url"`
}

type applicantDetail struct {
	ID        int                    `json:"id"`
	Candidate candidateDetail        `json:"candidate"`
	Stage     json.RawMessage        `json:"stage"`
	Status    json.RawMessage        `json:"status"`
	CreatedAt string                 `json:"created_at"`
	Raw       map[string]any         `json:"-"`
}

type candidateDetail struct {
	ID          int         `json:"id"`
	FirstName   string      `json:"first_name"`
	LastName    string      `json:"last_name"`
	Email       string      `json:"email"`
	Mobile      string      `json:"mobile"`
	Phone       string      `json:"phone_number"`
	Resume      *attachment `json:"resume"`
	CoverLetter *attachment `json:"cover_letter"`
	Portfolio   *attachment `json:"portfolio"`
}

func (a applicantDetail) displayName() string {
	name := a.Candidate.FirstName + " " + a.Candidate.LastName
	if name == " " {
		return fmt.Sprintf("applicant-%d", a.ID)
	}
	return name
}

func (c *client) getApplicant(id int) (*applicantDetail, error) {
	body, _, err := c.do("GET", fmt.Sprintf("/api/applicants/%d", id))
	if err != nil {
		return nil, err
	}
	var det applicantDetail
	if err := json.Unmarshal(body, &det); err != nil {
		return nil, fmt.Errorf("decode applicant %d: %w", id, err)
	}
	_ = json.Unmarshal(body, &det.Raw)
	return &det, nil
}
