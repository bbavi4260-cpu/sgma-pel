# V33 Validation

V33 is a direct copy of V32 with one wire-level change: the retained
`GetPlayerTCStats` response now includes a present empty
`AccountInfoWithTCStats.detailed_stats` message.

| Check | Expected result | Validation result |
|---|---|---|
| TC stats | `0a070881c2d72f2a00` | Passed. |
| Player stats | V32 three-mode 21-byte response | Passed unchanged. |
| Match History | Empty `CSGetMatchStatsListRes` | Passed unchanged. |
| Hide-avatar responses | V25 exact 14-byte/12-byte payloads | Passed unchanged. |
| Profiles | Non-empty Login and later profile lists | Passed, 1,938 and 1,817 bytes. |
| Source inventory | Exact `GetBackpack` size | Passed, 841,109 bytes. |
| Package hygiene | No logs, state, PID, or bytecode files | Passed before packaging. |

The purpose of this package is narrow: validate the non-null nested Clash-Squad
statistics object required by the screenshot-confirmed `PlayCsVfx` path. It does
not claim to resolve the separate Character index exception.
