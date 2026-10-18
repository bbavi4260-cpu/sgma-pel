# Clean Rebuild Validation

The clean source-first implementation passed local syntax, contract, and live
HTTP smoke checks on 2026-08-23.

| Check | Result |
|---|---|
| Python and shell syntax | Passed. |
| Canonical selected state | One selected profile, avatar `102000004`, with selected-items synchronized to that profile. |
| V25 hide-avatar continuity | `LoginGetDesc` body is exactly `420c0a0497cbd1301204d7c69430`. |
| Profile response | `LoginGetProfile` is exactly `0a090a070884cbd130300112050881c2d72f1a00`. |
| Backpack response | `GetBackpack` is exactly `12050884cbd1301a090884cbd13010012001`. |
| Preserved `versioninfo` | 10 bytes; SHA-256 `9820c3154fd1735ce8482ebe3feefb178cd3c118fc963acf75b4a9874a306c75`. |
| Preserved `fileinfo` | 4,791 bytes; SHA-256 `971c668cad2c3f43b373561a5caf5fbbdcd7233b4a6de9f4aaf23856468a9dfd`. |
| Preserved `config/avatar` bundle | 28,327 bytes; SHA-256 `4c1d09e69f4b09bd4f668f356b7452f69a4b87100e0de02b04ac852104617f02`. |
| V26/V27 state experiments | Excluded from source code. |
| FreeFire cosmetic fixtures | Excluded from source code. |

The preserved resource routes are passive compatibility endpoints. They do not
claim to schedule the missing client resource descriptor, asset indexer, recipe,
or wardrobe-definition layers.
