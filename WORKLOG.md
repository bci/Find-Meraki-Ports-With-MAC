# Work Log

Newest first. Absolute dates only. Earlier history (2026-02-13 to 2026-03-03) is in
`CHANGELOG.md` and the git log; this file starts on 2026-10-16.

## 2026-10-24

### ISSUES.md added (bookkeeping profile)

- Kent made `ISSUES.md` the fourth bookkeeping file (claude-config FEAT-0024). It was
  seeded from what this repo already records as open: 7 entries, each citing its source.
  Nothing was invented.
- The bookkeeping block in `CLAUDE.md` was re-synced from claude-config.
## 2026-10-16

### Code review (branch `code-review`)

Second half of the overnight run: a senior-engineer review of all first-party code, fixing what
was clearly wrong and safe, and writing up the rest. Full report, with every finding, its
evidence and the questions: `docs/code-review/2026-10-16.md`.

**Fixed (8):** MAC bracket ranges (`[1-4]`) never matched — the README's own example found
nothing; the CLI's live-MAC-table branch didn't flag AGGR uplinks; reflected XSS in `/topology`
(the comment said `strings.NewReplacer` "handles safely" — it doesn't escape); stored XSS in the
topology tooltip via DHCP hostnames; `/ws/logs` accepted every origin, so any open website could
read the log stream; `exitWithError` used the message as a format string; `copyright_test.go`'s
three gates had checked zero files since they were written; `build.ps1` stamped local time as UTC.

**Why these and not more.** The bar was "clearly wrong, safe, surgical". Each fix has a test that
failed first, except the CLI uplink flag (inside `main()`, so it was checked against a fake API
instead) and the two script/JS fixes (checked in pwsh and node). Things that would change what a
user sees in a way they might rely on — which hostname IP mode shows, error reporting for "All
Networks", `--port` matching, the `--verbose` stdout line — were left as questions, as were
structural changes (merging the duplicated CLI/web lookup, embedding `static/`, web-server
authentication). The duplication is the root cause of one of the bugs, so it is the first thing
worth doing next, behind a fake-API end-to-end test.

`features.yaml`: FEAT-0002 and FEAT-0020 moved to `complete`, FEAT-0011 to `in-test`, with notes.

### Documentation pass (branch `overnight-docs`)

Unattended overnight pass, asked for by Kent: bring the code up to `CLAUDE.md` §11 (complete
documentation in each language's own standard) and make the prose docs match the code, without
changing behaviour.

**What was documented.** GoDoc on every declaration in every `.go` file, tests included: 299
declarations (funcs, methods, types, struct fields, vars, consts), of which 148 had no comment
before. Several existing comments were corrected rather than kept as they were, because they
were wrong: `resolveEnvFile` said it created the file (main does); `ResultRow.IsUplink` said the
value came from the link-layer topology (it has come from the port statuses API since 1.3.0);
`doRequest` said "exponential" backoff (it is linear). JSDoc (`@module`, `@typedef`, every
method and property) on `static/js/app.js` and `static/js/topology.js`. Google Shell Style
headers on `build.sh` and `release.sh` (neither has functions). Comment-based help on
`build.ps1` and `test-live-clients.ps1`; `Get-Help -Full` now works for both — `build.ps1` needed
a blank line after its shebang, or PowerShell merges the two comment blocks and ignores the help.

**Why the comments say what they say.** The goal was the *why* a reader cannot recover from the
code, so comments call out the things that surprised me while reading: the CLI and web lookups
are two parallel copies of the same loop; the web path smuggles a network **ID** through
`Config.NetworkName`; static files are served relative to the working directory; the manufacturer
lookup goes to a third-party service; several `pkg/meraki` types and functions are left over from
the LLDP/topology uplink approach and are unused.

**How "no behaviour change" was checked.** Go: every `.go` file's token stream (go/scanner, comments
dropped) is identical to `pre-docs`. JS, shell, PowerShell: the diff against `pre-docs` adds only
comment and blank lines (the only removed lines are comments moved into the PowerShell help block).
Build, vet, `go test -race`, golangci-lint (0 issues) and shellcheck all pass, as they did before.

**Prose docs.** README: added the API endpoints the code actually calls, the `--retry`,
`--mac-table-poll` and `--dns-servers` flags, the AggrPorts and Uplink columns, a test/lint
section, and three warnings found while reading — run the web UI from the repo root, the web
server has no authentication and hands out the `.env` key, and `--port` matches substrings.
Removed two claims that were not true: "IEEE OUI database" (it is api.macvendors.com) and "Alert
Monitoring" (`/api/alerts` is a stub). `CHANGELOG.md` ended with a stray editor-tool fragment
(`</content><parameter name="filePath">c:\Users\...`); removed. Created `AGENTS.md`, `VERSIONS.md`
and `features.yaml` (21 features, reconstructed and marked INFERRED).

**Evidence for features.yaml.** Nothing was run against the real Meraki API. To see the CLI work at
all, it was run against a small local fake API (`MERAKI_BASE_URL=http://127.0.0.1:.../api/v1`):
both `--mac` and `--ip` produced the expected rows. That run is also how two bugs were confirmed —
recorded as `in-dev` rather than fixed here, because this pass changes comments only:

- bracket ranges in MAC patterns never match (the `-` is stripped as a separator), so the README's
  own `[1-4][0-f]` example finds nothing (FEAT-0002);
- the CLI's live-MAC-table branch does not flag an AGGR port as an uplink when its member is one,
  unlike the other two branches and the web path (FEAT-0011);
- and, by experiment, `copyright_test.go` checks no files at all — the walk skips `.` itself as a
  "hidden" directory — so the GPL-header and no-inline-JS gates have never run (FEAT-0020).

Skipped: CSS (no documentation standard in §11; both files already have header comments), the
YAML workflow, `go.sum`, the PNG, and the historical reports (`COMPLETION_REPORT.md` and
friends), which describe past sessions and are left as written.
