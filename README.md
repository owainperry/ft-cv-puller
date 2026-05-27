# ft-cv-puller

A small Go CLI that pulls every applicant CV (plus cover letter and portfolio, when present) for a given Freshteam job posting and saves them to disk alongside the applicant's raw metadata as JSON.

## What it does

For a given Freshteam job posting (role) ID, the tool:

1. Looks up the job posting and prints its title.
2. Pages through all applicants on that posting.
3. For each applicant, downloads their resume, cover letter, and portfolio attachments (whichever are present) and writes a JSON file containing the full applicant record.

Files are written to `<out>/<role>/` and named `<applicant-id>_<firstname>_<lastname>_<kind><ext>`, e.g. `12345_jane_doe_cv.pdf`. Existing files are skipped on re-run, so the tool is safe to run repeatedly.

## Install

Download a prebuilt binary for your platform from the [Releases](../../releases) page, or build from source:

```sh
make build         # produces ./ft-cv-puller for the host platform
```

## Getting a Freshteam API token

1. Sign in to your Freshteam account as a user with **Recruit** access to the job postings you want to pull.
2. Open **Profile Settings** (top-right avatar) → **API Key**.
3. Copy the token shown. You will also need your Freshteam subdomain, e.g. `acme.freshteam.com`.

The token must have permission to read job postings and applicants on the Recruit module.

## Configuring the token and domain

The tool needs two values: the Freshteam **domain** (e.g. `acme.freshteam.com`) and an **API key**. Each can be supplied in three ways. Precedence, highest first:

1. CLI flag — `-domain`, `-key`
2. Environment variable — `FRESHTEAM_DOMAIN`, `FRESHTEAM_API_KEY`
3. Config file — `~/.ft-cv-puller`

### Option 1 — CLI flags (quick, but the token appears in shell history)

```sh
./ft-cv-puller -role 5000154941 -domain acme.freshteam.com -key YOUR_TOKEN_HERE
```

### Option 2 — Environment variables

```sh
export FRESHTEAM_DOMAIN=acme.freshteam.com
export FRESHTEAM_API_KEY=YOUR_TOKEN_HERE
./ft-cv-puller -role 5000154941
```

### Option 3 — Config file at `~/.ft-cv-puller` (recommended)

Create the file with `key=value` lines:

```
# ~/.ft-cv-puller
domain=acme.freshteam.com
key=YOUR_TOKEN_HERE
```

Then lock it down so other users on the machine can't read your token:

```sh
chmod 600 ~/.ft-cv-puller
```

The tool warns on startup if the file is group- or world-readable.

**Backwards compatibility:** if the file contains no `=` characters, its entire contents are treated as a bare API token (and `-domain` or `FRESHTEAM_DOMAIN` must then supply the domain).

## Usage

```
Usage: ft-cv-puller -role <id> -domain host [-out dir] [-key token] [-dry-run] [-debug]
```

| Flag        | Default     | Description                                                                 |
|-------------|-------------|-----------------------------------------------------------------------------|
| `-role`     | _(required)_| Freshteam job posting ID                                                    |
| `-domain`   | _(required)_| Freshteam subdomain host, e.g. `acme.freshteam.com`                         |
| `-key`      | _(required)_| Freshteam API token                                                         |
| `-out`      | `./cvs`     | Output directory; files are written under `<out>/<role>/`                   |
| `-dry-run`  | `false`     | List applicants and exit; download nothing                                  |
| `-debug`    | `false`     | Log where domain/key were loaded from and dump the first applicant's raw JSON to `<out>/<role>/_debug_first_applicant.json` |
| `-version`  |             | Print version and exit                                                      |

### Examples

List applicants without downloading anything:

```sh
./ft-cv-puller -role 5000154941 -dry-run
```

Pull every CV for a role into `./cvs/5000154941/`:

```sh
./ft-cv-puller -role 5000154941
```

Pull into a custom directory and dump the first applicant's raw JSON for inspection:

```sh
./ft-cv-puller -role 5000154941 -out ~/recruiting/2026-q2 -debug
```

## Finding the role ID

You can list all published job postings via the API:

```sh
TOKEN=$(grep '^key=' ~/.ft-cv-puller | cut -d= -f2-)
curl -sS -H "Authorization: Bearer $TOKEN" \
  "https://acme.freshteam.com/api/job_postings?status=published"
```

Each posting object includes an `id` — that's what you pass to `-role`.

## Output layout

```
cvs/
└── 5000154941/
    ├── 12345_jane_doe.json          # full applicant record
    ├── 12345_jane_doe_cv.pdf
    ├── 12345_jane_doe_cover.pdf
    ├── 12346_john_smith.json
    └── 12346_john_smith_cv.docx
```

Re-running skips files that already exist, so the tool is safe to run incrementally as new applicants arrive.

## Building from source

Requires Go 1.22+.

```sh
make build          # build for host platform
make test           # run tests
make release        # cross-compile all platforms into ./dist/
```

The `release` target produces tarballs/zips for linux, darwin, and windows (amd64 + arm64 where applicable), along with `SHA256SUMS`.

## Releases

Every push to `main` triggers the `release` GitHub Action, which builds cross-platform binaries and publishes a GitHub Release tagged `v0.1.<run-number>`.
