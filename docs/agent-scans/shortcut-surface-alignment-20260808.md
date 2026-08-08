# Shortcut surface alignment Agent scan

- generated_at: `2026-08-08T14:56:13`
- source: current `go run ./cmd shortcut list --all --mock --format json`
- fixture policy: runtime JSON is held in memory and not saved; this file is Markdown evidence only
- result: **PASS**

| surface | count |
|---|---:|
| runtime public | 376 |
| committed catalog | 376 |
| Mono Skill total | 376 |

| service | catalog | Skill |
|---|---:|---:|
| `aitable` | 92 | 92 |
| `attendance` | 35 | 35 |
| `calendar` | 20 | 20 |
| `chat` | 98 | 98 |
| `contact` | 14 | 14 |
| `devapp` | 20 | 20 |
| `ding` | 4 | 4 |
| `doc` | 45 | 45 |
| `drive` | 7 | 7 |
| `mail` | 10 | 10 |
| `minutes` | 7 | 7 |
| `oa` | 7 | 7 |
| `report` | 2 | 2 |
| `sheet` | 2 | 2 |
| `todo` | 11 | 11 |
| `wiki` | 2 | 2 |

## Findings

- runtime public set, committed catalog, and Skill counts are identical
