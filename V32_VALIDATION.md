# V32 Profile History and Player-Statistics Correction

V32 is a direct copy of the phone-verified V31 package. Its only protocol
changes are the two routes that the V31 phone log recorded as 404 after the
lobby and Profile base panel had already rendered.

| Route | Confirmed request-to-response contract | V32 response |
|---|---|---|
| `GetMatchStatsHistory` | `AccountIDReq` → `CSGetMatchStatsListRes`; repeated `match_stats_list` is field 1 | Zero-byte protobuf response representing an empty declared list; no match records invented. |
| `GetPlayerStats` | `CSGetPlayerStatsReq` → `CSGetPlayerStatsRes`; fields 1–3 are solo, duo, quad `AccountInfoWithStatsToClient` messages | Three non-null nested messages. Each contains only the canonical account ID at nested field 1; counters stay zero and are omitted. |

The V25 non-empty hide-avatar descriptor, V29 source-default account and
inventory projection, V30 roster/skill-cardinality correction, V31
`GetPlayerTCStats`, V31 `GetBattleTag`, and preserved local Sigma resource
files remain unchanged.

Character remains an independent, non-regressed issue. The matched client
requires its separate avatar-skill definition dictionaries for Character-card
binding; no authentic local avatar-skill definition resource is available in
the preserved Sigma files or the nearby source.
