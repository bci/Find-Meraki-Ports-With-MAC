# Versions

What shipped, newest first. Created 2026-10-16 from `CHANGELOG.md` and the git tags; the
user-facing detail stays in `CHANGELOG.md`, this file records which commit each version is.

| Version | Date | Tag / commit | Notes |
| --- | --- | --- | --- |
| Unreleased | 2026-10-16 | branches `overnight-docs`, `code-review` | Documentation pass (no behaviour change), then review fixes: MAC bracket ranges, CLI AGGR uplink flag, topology XSS, WebSocket origin check, `%` in fatal errors, copyright CI gate. See WORKLOG.md and docs/code-review/2026-10-16.md. |
| 1.3.2 | 2026-03-03 | no tag; `739924a` | `--env` flag, `~/.env.find-mac` default and auto-created stub. The build scripts still say `1.3.1`. |
| 1.3.1 | 2026-03-02 | tag `v1.3.1` → `a874d81` (2026-03-03) | Split `main.go` into focused files; `resolve_test.go`. |
| 1.3.0 | 2026-03-02 | no tag | `--test-data` demo mode; uplink detection via switch port statuses. |
| 1.2.0 | 2026-02-26 | tag `v1.2.0` → `e6b75e6` (2026-02-28) | Interactive web UI, presets, port mode, DNS servers and `HOST_OVERRIDES`. |
| 1.1.0 | 2026-02-19 | no tag; merge `11af187` | `--ip` lookup and reverse-DNS hostnames. |
| 1.0.0 | 2026-02-13 | tag `v1.0.0` → `934bea1` | First release: MAC lookup CLI, wildcard patterns, CSV/text/HTML output. |

`CHANGELOG.md` links `v1.1.0`, `v1.3.0` and `v1.3.2` release tags that are not present in this
clone (they may or may not exist on GitHub; not checked).
