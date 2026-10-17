# AGENTS.md

Orientation for coding agents (and people) working in this repository. The rules for how to work
are in `CLAUDE.md`; what the tool does for its users is in `README.md`. This file is the map.

## What this is

A Go command-line tool that asks the Cisco Meraki Dashboard API which switch and port a MAC
address (or an IP address, resolved to a MAC) was seen on. It works with Meraki MS switches and
Meraki-managed Cisco Catalyst switches, the latter through the live MAC table API. The same binary
has an interactive mode (`--interactive`, or `--test-data` for canned demo data) that serves a
local browser UI.

Go module name: `Find-Meraki-Ports-With-MAC`. `go.mod` declares `go 1.21`; dependencies are
`gorilla/mux`, `gorilla/websocket` and `joho/godotenv`.

## Layout

| Path | What it holds |
| --- | --- |
| `main.go` | Entry point: `.env` loading, flags, `Config`, CLI lookup loop, `--help`/`--version` text |
| `resolve.go` | Web-path lookup (`resolveDevices`, `processSwitchesForResolution`) and AGGR/uplink helpers shared with the CLI |
| `web.go` | Web server start-up and routes, WebSocket log hub, the main HTML page |
| `web_handlers.go` | `/api/*` and `/topology` handlers that call the real Meraki API |
| `web_demo.go` | `--test-data` handlers (fixed demo data, no Meraki calls) |
| `oui.go` | Manufacturer lookup via `api.macvendors.com`, cached per process |
| `pkg/meraki/` | Meraki API client: pagination, 429 retry, live MAC/ARP tables, host overrides, reverse DNS |
| `pkg/macaddr/` | MAC normalisation, formatting and wildcard/bracket pattern matching |
| `pkg/filters/` | Switch detection and `--switch` / `--port` filters |
| `pkg/output/` | `ResultRow` and the CSV / text / HTML writers |
| `pkg/logger/` | Levelled logger (`<RFC3339> [LEVEL] message`) |
| `static/js/app.js`, `static/css/style.css` | Main web UI (served from `./static/` relative to the working directory) |
| `static/js/topology.js`, `static/css/topology.css` | D3 topology page |
| `*_test.go`, `pkg/*/*_test.go` | Unit tests (no network access to Meraki) |
| `copyright_test.go` | CI gate: GPL header on every `.go` file; no inline CSS/JS in Go templates |
| `Makefile`, `build.sh`, `build.ps1` | Test, lint and cross-platform builds |
| `release.sh` | Builds and **publishes** a GitHub release (force-moves the tag). Do not run. |
| `test-live-clients.ps1` | Developer probe for an Early Access Meraki endpoint; calls the real API. Do not run. |
| `.github/workflows/test.yml` | CI: tests on Go 1.21–1.23 × Linux/macOS/Windows, golangci-lint, copyright gate, builds |
| `docs/prompts/` | The specification prompts the code was built from |
| `CHANGELOG.md` | User-facing release notes (Keep a Changelog) |
| `WORKLOG.md`, `VERSIONS.md`, `features.yaml` | Bookkeeping per `CLAUDE.md` |
| `COMPLETION_REPORT.md`, `VERSIONING.md`, `VERSION_IMPLEMENTATION.md`, `SAMPLE_RUN.md`, `.github/DOCUMENTATION_SUMMARY.md` | Historical reports from earlier sessions; not kept current |

## Commands

```sh
go build ./...                 # compile everything
go vet ./...
go test -race ./...            # what CI runs (CI adds -coverprofile)
golangci-lint run ./...        # CI uses the latest golangci-lint
make test                      # go test -v ./...
make lint                      # go vet + golangci-lint
./build.sh                     # test + vet + golangci-lint, then ./findmac
./build.sh --package           # ... plus six static binaries in ./bin/
go run . --help
go run . --interactive --test-data   # demo UI, no API key; opens your browser
```

On this machine Go is pinned to 1.26.7 (`GOTOOLCHAIN`); do not change `go.mod`'s `go` line.

To exercise the CLI end to end without Meraki, point `MERAKI_BASE_URL` at a local fake server
(e.g. `http://127.0.0.1:PORT/api/v1`) and set any non-empty `MERAKI_API_KEY`.

## Conventions

- Every `.go` file starts with the GPL v3 header beginning `// Copyright (C) 2025 Kent Behrends`;
  `copyright_test.go` enforces it.
- No inline CSS or JS in Go string templates: put it in `static/` (also enforced by
  `copyright_test.go`). Avoid writing the literal opening style or bare script tags anywhere in a
  `.go` file, including comments, or that test fails.
- GoDoc on every declaration, JSDoc in `static/js/`, Google Shell Style headers on shell scripts,
  comment-based help on PowerShell scripts (`CLAUDE.md` §11).
- CLI and web lookups are parallel implementations (`main.go` loop vs.
  `processSwitchesForResolution`); a behaviour fix in one usually belongs in the other too.
- The version string lives in four places that are edited by hand: `Makefile`, `build.sh`,
  `build.ps1`, `release.sh` (CI hard-codes `1.0.0`).

## Guardrails

- Never call the real Meraki API from tests or automation, and never commit an API key. `.env`
  is git-ignored; the user's config lives in `~/.env.find-mac`.
- Do not run `release.sh` (it pushes a force-moved tag and replaces a GitHub release) or
  `test-live-clients.ps1` (real API).
- Running the binary creates `~/.env.find-mac` if it is missing and appends to
  `Find-Meraki-Ports-With-MAC.log` in the working directory; set `HOME` and `--log-file` when
  testing.
- `--interactive` opens the system browser and exposes the `.env` API key to anything that can
  reach the port; keep `--web-host localhost`.

## Not done yet

- `/api/alerts` and `/api/logs` are stubs returning empty lists.
- `pkg/meraki` still carries unused types/functions from the earlier LLDP/topology uplink
  approach (`BuildUplinkPortSet`, `LLDPCDPData`, `SwitchPortFull`).
- No end-to-end test of the lookup loops; they are only exercised by hand.
