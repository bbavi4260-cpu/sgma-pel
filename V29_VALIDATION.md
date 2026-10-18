# V29 Source-Default Validation

V29 directly imports the nearby-version source defaults into the Sigma
protobuf envelopes. Local syntax, deterministic contract, and live HTTP tests
passed on 2026-08-23.

| Check | Result |
|---|---|
| Source default profiles | 51 imported; 50 visible after the source `102000008` filter. |
| Selected Adam profile | `102000004`, `skin_color=5`, clothes `203000001,211000000,204000001,205000001`. |
| Source inventory | 29,302 ownership records imported from source `items.json`; SHA-256 `626aa453dffba112af7a13f4b713e1c42585837590965dbc178c0a511fc8b787`. |
| `LoginGetProfile` | 1,793 bytes in live test. |
| `GetProfiles` | 1,781 bytes in live test. |
| `GetBackpack` | 841,109 bytes in live test. |
| `GetStore` | 17-byte source free-character sample. |
| Platform friend routes | `GetPlatformFriends` and `GetPlatformFriendIDs` now return HTTP 200 using source-defined empty response models. |
| `ChangeClothes` | Parses source field layout, mutates the target profile, mirrors selected-items, and persists. |
| V25 hide-avatar continuity | `LoginGetDesc` remains `420c0a0497cbd1301204d7c69430`. |

V29 still does not manufacture a Sigma wardrobe recipe mapping, material
reference, asset-indexer record, or optional-resource cache descriptor. It is
therefore a direct source-state compatibility test, not a guarantee that every
source inventory ID has a renderable Sigma resource.
