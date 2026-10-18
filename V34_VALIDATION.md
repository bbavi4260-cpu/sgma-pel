# V34 Validation

V34 is a direct copy of V33 with two additions tied to the same-run re-login
log: `SetPregameShowChoices` and `UpdateSocialBasicInfo`.

| Check | Expected result | Validation result |
|---|---|---|
| Pregame choices | Declared empty acknowledgement | Passed. |
| Social Basic Info | `0a050881c2d72f` | Passed. |
| TC stats | V33 `0a070881c2d72f2a00` | Passed unchanged. |
| Player stats | V32 three-mode response | Passed unchanged. |
| Match History | Empty declared list | Passed unchanged. |
| Hide-avatar | V25 exact payloads | Passed unchanged. |
| Profiles | Non-empty login and later lists | Passed: 1,938 and 1,817 bytes. |
| Source inventory | Exact backpack response | Passed: 841,109 bytes. |
| Package hygiene | No logs, state, PID, or bytecode | Passed before packaging. |

The phone test must verify whether the explicit re-login popup disappears. V34
does not claim to repair unrelated Character or resource-configuration issues.
