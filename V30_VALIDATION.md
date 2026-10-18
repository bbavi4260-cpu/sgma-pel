# V30 Character/Profile Skill-Projection Validation

V30 preserves V29’s working source-default lobby and Vault state and changes
only the post-login Character/Profile inputs identified from the genuine V29
phone run.

| Correction | Validation result |
|---|---|
| `CSGetSkillListRes.skills` | 51 unique source-derived available skills are emitted alongside the account ID. |
| Login profile roster | 51 source profiles; 1,938-byte live response. |
| Later profile roster | The same 51 profiles; 1,817-byte live response. |
| Cardinality split | Removed: V29 had 51 login profiles but 50 later profiles due to an unrelated nearby-version filter. |
| Lobby/Vault source inventory | Preserved: `GetBackpack` remains 841,109 bytes. |
| Hide-avatar continuity | Preserved: `420c0a0497cbd1301204d7c69430`. |

The correction targets the V29 `ArgumentOutOfRangeException` in the
Character/Profile path. It does not alter the source profile roster, selected
Adam clothes/skin, source item inventory, resource routes, or the working
lobby/Vault behavior.
