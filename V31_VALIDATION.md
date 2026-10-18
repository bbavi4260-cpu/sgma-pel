# V31 Lobby-Stability Endpoint Validation

The V30 phone log showed these two 404 responses immediately before the client
displayed “Error occurred when connecting to server, please re-login.” V31
changes only those routes.

| Route | V30 phone result | V31 live result |
|---|---|---|
| `GetPlayerTCStats` | HTTP 404 | HTTP 200, 7-byte non-null `AccountInfoWithTCStats` response: `0a050881c2d72f`. |
| `GetBattleTag` | HTTP 404 | HTTP 200, zero-byte valid response with declared repeated collections empty. |
| `GetBackpack` | Working V29/V30 source data | HTTP 200, 841,109-byte source inventory response retained. |
| `LoginGetDesc` | Working V25 fix | Preserved: `420c0a0497cbd1301204d7c69430`. |

V31 retains the V29 source roster, selected Adam clothes and skin, source item
corpus, V30 available-skill list, and V30 matching 51-profile cardinality. It
does not change rendering or cosmetic data.
