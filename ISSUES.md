# Issues

Open problems and pending decisions for this repository, newest first. The format and rules are
in the bookkeeping profile in `CLAUDE.md` ("`ISSUES.md`: open problems and pending decisions").

- **Statuses:** `OPEN`, `RESOLVED`, or `WONTFIX`. A closed entry is never deleted; it gains a
  `Resolved` or `Won't fix` line.
- **Severity:**
  - `HIGH`: wrong now, enforceable now, or has a fixed date;
  - `MEDIUM`: a real gap with no immediate clock;
  - `LOW`: tidy-up.

---

## OPEN — Bracket ranges in MAC patterns never match, including the README's own example

- **Date:** 2026-10-16
- **Area:** `pkg/macaddr` (FEAT-0002); README pattern examples
- **Severity:** HIGH. It is broken now: a documented pattern silently finds nothing.
- **Issue:** "separators are stripped before the pattern is compiled, so the `-` inside `[1-4]`
  is removed and the README example `08:f1:b3:6f:9c:[1-4][0-f]` never matches". Exact formats
  and `*` work and are unit-tested.
- **Source:** `features.yaml` FEAT-0002 notes ("verified 2026-10-16"); `WORKLOG.md`,
  2026-10-16 "Documentation pass", "Evidence for features.yaml". Confirmed by a CLI run against
  a local fake Meraki API; recorded as `in-dev`, not fixed, because that pass changed comments
  only.
- **Options:** keep `-` inside brackets when stripping separators (with a test using the README
  example), or drop bracket ranges from the README until fixed.

## OPEN — CLI does not flag an AGGR port as an uplink when its member port is an uplink

- **Date:** 2026-10-16
- **Area:** the CLI lookup loop's live-MAC-table branch in `main.go` (FEAT-0011)
- **Severity:** HIGH. Wrong now: a MAC seen on an uplink can be reported as an access port.
- **Issue:** "a MAC found in the live MAC table on an AGGR port whose member is an uplink is NOT
  flagged ... because that branch checks the AGGR ID itself rather than its members". The other
  two CLI branches and the web path handle it.
- **Source:** `features.yaml` FEAT-0011 notes; `WORKLOG.md`, 2026-10-16 "Documentation pass",
  "Evidence for features.yaml". Seen against the local fake API only, not the real Meraki API.
- **Options:** make that branch check the AGGR members as the other branches do, with a test
  reproducing the fake-API case.

## OPEN — The GPL-header and no-inline-asset tests pass without checking any file

- **Date:** 2026-10-16
- **Area:** `copyright_test.go` (FEAT-0020); CI
- **Severity:** HIGH. The gate is broken now: it reports success and has never run.
- **Issue:** "The tests pass but check nothing: the directory walk skips "." itself (it is
  treated as a hidden directory), so no file is ever opened". WORKLOG: "the GPL-header and
  no-inline-JS gates have never run".
- **Source:** `features.yaml` FEAT-0020 notes; `WORKLOG.md`, 2026-10-16 "Documentation pass",
  "Evidence for features.yaml" ("by experiment").
- **Options:** exempt the walk root from the hidden-directory skip, then fix any files the
  restored gate flags (it may find real failures, since it has never run).

## OPEN — Most features are unverified against the real Meraki API

- **Date:** 2026-10-16
- **Area:** `features.yaml` (FEAT-0001, 0004–0010, 0012–0016, 0019 are `in-test`)
- **Severity:** MEDIUM. A real gap in evidence, with no immediate clock.
- **Issue:** Every entry was reconstructed (INFERRED) and "Nothing was run against the real
  Meraki API, so anything whose correctness depends on real API responses is `in-test`, not
  `complete`, until Kent confirms it."
- **Source:** `features.yaml` header comment (lines 26–30) and the `in-test` entries' notes;
  `WORKLOG.md`, 2026-10-16 "Documentation pass", "Evidence for features.yaml".
- **Options:** Kent runs the CLI and web UI against a real network and moves each confirmed
  feature to `complete`. **Kent decides and verifies**; it needs a real API key.

## OPEN — Alerts and log-history endpoints are stubs: wanted or deferred?

- **Date:** 2026-10-16
- **Area:** `/api/alerts`, `/api/logs` in the web server (`web.go`, `web_handlers.go`) (FEAT-0021)
- **Severity:** LOW. The stubs are harmless; this is a scope decision.
- **Issue:** "Only stubs exist; both return empty lists and nothing calls them. Kent to decide
  whether this is wanted or should be deferred." The README's "Alert Monitoring" claim was
  removed on 2026-10-16.
- **Source:** `features.yaml` FEAT-0021 notes (status `planned`, INFERRED); `WORKLOG.md`,
  2026-10-16 "Documentation pass", "Prose docs".
- **Options:** implement them, mark FEAT-0021 `deferred`, or remove the stubs. **Kent decides.**

## OPEN — Version numbers disagree across build files, and GoVersion is never injected

- **Date:** 2026-10-16
- **Area:** `Makefile`, `build.sh`, `build.ps1`, `.github/workflows/test.yml` (FEAT-0018)
- **Severity:** LOW. `--version` works; the numbers it can print are inconsistent.
- **Issue:** "GoVersion is never injected and always prints go1.21; the version number differs
  between Makefile (1.2.0), the build scripts (1.3.1) and CI (1.0.0)." VERSIONS.md adds that
  1.3.2 shipped while "The build scripts still say `1.3.1`."
- **Source:** `features.yaml` FEAT-0018 notes ("Seen working 2026-10-16"); `VERSIONS.md`,
  1.3.2 row.
- **Options:** take the version from one place (for example the latest tag) in all three, and
  inject the Go version at build time.

## OPEN — CHANGELOG links release tags that are not in this clone

- **Date:** 2026-10-16
- **Area:** `CHANGELOG.md`, git tags
- **Severity:** LOW. Possibly broken links; unchecked.
- **Issue:** "`CHANGELOG.md` links `v1.1.0`, `v1.3.0` and `v1.3.2` release tags that are not
  present in this clone (they may or may not exist on GitHub; not checked)."
- **Source:** `VERSIONS.md`, note below the table (created 2026-10-16).
- **Options:** check the tags on GitHub; create the missing tags at the commits VERSIONS.md
  names, or correct the links.
