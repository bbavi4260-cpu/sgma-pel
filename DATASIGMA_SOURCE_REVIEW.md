# `datasigma.zip` source review

## What is present

The archive contains 6,033 `.cs` files under `data sigma/`. It gives useful **client data-model declarations**. For example, `EProtocol.Proto.MATCHMAKING` is `3`; `EMatch.MatchMode_CASUAL` and `EMatch.GameMode_BattleRoyale` are both `1`. `MatchmakingStartReq` declares `map_ids`, `game_mode`, `match_mode`, `available_maps`, platform/client fields, and optional workshop fields. `MatchmakingSussNtf` declares the match ID, server address, secret, prepare token, numeric map ID, modes, and other handoff fields. `ProtoReq` only declares `cmd` and `data`.

The map model `COW/MapConfigData.cs` declares map name, map ID, mode ID, config ID, and resource references. Its `DEFAULT_MAP_ID` constant is `1`, but the archive contains no map catalogue rows mapping names such as `PARADISE` or `AZORES` to real client numeric IDs. The parsing and ID-conversion methods are also decompiler stubs.

## What is missing

The archive is source declarations, not the running server or map content. It contains no `.proto` files, numeric map catalogue, map bundles, collision/world data, transport captures, or working server implementation. The client-side methods central to this error are empty bodies in the decompile: `LobbyServiceConnectionHandler.OnMsgHasLatestGameInfo`, `OnMsgMatchMaking`, and `MatchSuccessLoading`, plus `UIModelMatch.RequestStartMatchMakingInternal`.

The declarations therefore help identify the **shape** of matchmaking fields, but they do not establish protobuf field tags/serialization, TCP frame boundaries, encryption/session setup, valid map-ID mappings, game-server preparation behavior, or the UDP real-time gameplay protocol. Fabricating these parts would be guesswork and could keep the client on “Reading game info, please wait” or break login.

## Effect on V38

The SQLite map catalogue intentionally keeps the five names supplied in the account config as strings and does not claim they are client numeric map IDs. V38’s TCP listener remains a diagnostic socket, and UDP only logs datagram metadata. Neither sends a matchmaking or gameplay response. This is the furthest safe implementation supported by the supplied source; a real playable match still needs a verified client/server exchange or the missing protocol implementation and map assets.
