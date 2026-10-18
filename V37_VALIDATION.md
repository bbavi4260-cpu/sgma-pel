# V37 Validation

V37 is a direct copy of V36 with only the Personal Show response narrowed to the
field shape that retains V35 rendering compatibility.

| Check | Expected result | Validation result |
|---|---|---|
| LoginData | Source level 100, exp 250000, 999999 wallet | Passed unchanged. |
| Backpack | Full source SelectedItems fields and source inventory | Passed unchanged. |
| Personal Show | Source basic identity + selected profile + empty field-10 DiamondCostRes | Passed. |
| Personal Show exclusions | No top-level ranking field 3 or EP-history field 5 | Passed. |
| Notification channel | Opaque Cmd=1/Cmd=2 confirmations, redacted log | Passed unchanged. |
| V25–V35 contracts | Hide-avatar, routes, stats, profile cardinality | Passed unchanged. |
| Package hygiene | No runtime logs, state, PID, or bytecode | Passed before packaging. |

The phone test must verify whether Profile renders again while retaining the
source account presentation values. Character and the separate Profile-close
look-at issue remain independent checks.
