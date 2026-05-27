package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const configFileName = ".ft-cv-puller"

var version = "dev"

type config struct {
	role   int
	out    string
	domain string
	key    string
	dryRun bool
	debug  bool
}

func main() {
	cfg, err := parseConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	client := newClient(cfg.domain, cfg.key)

	posting, err := client.getJobPosting(cfg.role)
	if err != nil {
		log.Fatalf("fetch job posting %d: %v", cfg.role, err)
	}
	fmt.Printf("Role %d: %s\n", cfg.role, posting.Title)

	applicants, err := client.listApplicants(cfg.role)
	if err != nil {
		log.Fatalf("list applicants: %v", err)
	}
	fmt.Printf("Found %d applicants\n", len(applicants))

	if cfg.dryRun {
		for i, a := range applicants {
			fmt.Printf("[%d/%d] id=%d %s\n", i+1, len(applicants), a.ID, a.displayName())
		}
		return
	}

	roleDir := filepath.Join(cfg.out, fmt.Sprintf("%d", cfg.role))
	if err := os.MkdirAll(roleDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", roleDir, err)
	}

	for i, summary := range applicants {
		full, err := client.getApplicant(summary.ID)
		if err != nil {
			log.Printf("[%d/%d] id=%d: fetch detail: %v", i+1, len(applicants), summary.ID, err)
			continue
		}

		if cfg.debug && i == 0 {
			raw, _ := json.MarshalIndent(full.Raw, "", "  ")
			debugPath := filepath.Join(roleDir, "_debug_first_applicant.json")
			_ = os.WriteFile(debugPath, raw, 0o644)
			fmt.Printf("debug: wrote %s\n", debugPath)
		}

		written, skipped, err := saveApplicant(roleDir, full, client)
		if err != nil {
			log.Printf("[%d/%d] %s: %v", i+1, len(applicants), full.displayName(), err)
			continue
		}
		fmt.Printf("[%d/%d] %s: wrote %v%s\n",
			i+1, len(applicants), full.displayName(),
			written, skippedSuffix(skipped))
	}
}

func parseConfig() (config, error) {
	var cfg config
	var showVersion bool
	flag.IntVar(&cfg.role, "role", 0, "Freshteam job posting ID (required)")
	flag.StringVar(&cfg.out, "out", "./cvs", "output directory")
	flag.StringVar(&cfg.domain, "domain", "", "Freshteam subdomain host, e.g. acme.freshteam.com (required; or set FRESHTEAM_DOMAIN env or domain= in ~/.ft-cv-puller)")
	flag.StringVar(&cfg.key, "key", "", "Freshteam API token (or set FRESHTEAM_API_KEY env or key= in ~/.ft-cv-puller)")
	flag.BoolVar(&cfg.dryRun, "dry-run", false, "list applicants and exit; do not download")
	flag.BoolVar(&cfg.debug, "debug", false, "dump the first applicant's raw JSON to <out>/<role>/_debug_first_applicant.json")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -role <id> -domain host [-out dir] [-key token] [-dry-run] [-debug]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Precedence for each of -domain and -key:")
		fmt.Fprintln(os.Stderr, "  flag > FRESHTEAM_DOMAIN / FRESHTEAM_API_KEY env > ~/.ft-cv-puller")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "~/.ft-cv-puller format (chmod 600):")
		fmt.Fprintln(os.Stderr, "  # comments allowed")
		fmt.Fprintln(os.Stderr, "  domain=acme.freshteam.com")
		fmt.Fprintln(os.Stderr, "  key=YOUR_TOKEN_HERE")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Backwards-compat: a file with no '=' is treated as a single bare token.")
		fmt.Fprintln(os.Stderr, "")
		flag.PrintDefaults()
	}
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if cfg.role == 0 {
		flag.Usage()
		return cfg, errors.New("-role is required")
	}

	fileCfg, filePath, err := loadConfigFile()
	if err != nil {
		return cfg, err
	}

	domain, domainSrc, err := resolveValue("domain", cfg.domain, "FRESHTEAM_DOMAIN", fileCfg, filePath)
	if err != nil {
		return cfg, err
	}
	cfg.domain = domain

	key, keySrc, err := resolveValue("key", cfg.key, "FRESHTEAM_API_KEY", fileCfg, filePath)
	if err != nil {
		return cfg, err
	}
	cfg.key = key

	if cfg.debug {
		fmt.Fprintf(os.Stderr, "debug: domain loaded from %s; API key loaded from %s\n", domainSrc, keySrc)
	}
	return cfg, nil
}

// resolveValue returns the value of `name` from (in priority order):
//   flag value > env var > config-file map. Returns the source for debug logging.
func resolveValue(name, flagValue, envVar string, fileCfg map[string]string, filePath string) (string, string, error) {
	if flagValue != "" {
		return flagValue, "flag", nil
	}
	if v := os.Getenv(envVar); v != "" {
		return v, "env (" + envVar + ")", nil
	}
	if v, ok := fileCfg[name]; ok && v != "" {
		return v, filePath, nil
	}
	return "", "", fmt.Errorf("no %s found. Set -%s, %s env, or %s= in %s",
		name, name, envVar, name, filePath)
}

// loadConfigFile reads ~/.ft-cv-puller and returns parsed entries.
// Format: `key=value` per line, with `#` comments and blank lines ignored.
// Backwards-compat: if no line contains `=`, the whole non-empty content is
// treated as a bare token and returned as {"key": <token>}.
// A missing file returns an empty map (not an error) — flag/env may suffice.
func loadConfigFile() (map[string]string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", fmt.Errorf("resolve home dir: %w", err)
	}
	path := filepath.Join(home, configFileName)

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]string{}, path, nil
		}
		return nil, path, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		fmt.Fprintf(os.Stderr, "warning: %s is group/world-readable (%v); run: chmod 600 %s\n",
			path, info.Mode().Perm(), path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, path, fmt.Errorf("read %s: %w", path, err)
	}

	cfg := map[string]string{}
	hasPair := false
	for lineNo, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.IndexByte(line, '='); idx > 0 {
			hasPair = true
			k := strings.TrimSpace(line[:idx])
			v := strings.TrimSpace(line[idx+1:])
			v = strings.Trim(v, `"'`)
			if k == "" {
				return nil, path, fmt.Errorf("%s:%d: empty key", path, lineNo+1)
			}
			cfg[k] = v
		}
	}

	if !hasPair {
		token := strings.TrimSpace(string(data))
		if token != "" {
			cfg["key"] = token
		}
	}

	return cfg, path, nil
}

func skippedSuffix(skipped []string) string {
	if len(skipped) == 0 {
		return ""
	}
	return fmt.Sprintf(" (skipped existing: %s)", strings.Join(skipped, ", "))
}
