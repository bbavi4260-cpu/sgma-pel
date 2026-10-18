# Build note

`match-server/battle_royale.go` is not part of this tested package. It belongs to an incomplete experimental BR branch and references fields that do not exist in the current CS match-node architecture (`cfg.brExperimental`, `joinPlayer.GameMode`, `Match.gameMode`, etc.).

Before building an older/merged checkout, remove that file:

```bash
rm -f match-server/battle_royale.go
bash termux_start_server.sh
```

This package keeps Battle Royale Palace at the TCP selector level but routes unsupported BR gameplay to the tested Contra Squad UDP node. Do not copy an experimental `battle_royale.go` back into `match-server` unless it is fully integrated and compiled against the current structures.
