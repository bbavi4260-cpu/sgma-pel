# Earlier `Assembly-CSharp` decompile: proto folder review

This is the first uploaded `Assembly-CSharp.dll_Decompiler.com.zip`, not the later `datasigma.zip`. The earlier archive does contain a `proto/` directory with C# message-model declarations. I should have distinguished these two archives more clearly in my prior summary.

`proto/AvatarProfile.cs` defines the avatar profile fields: `avatar_id`, `unlocked_level`, `skin_color`, `clothes`, `equiped_skills`, `is_selected`, and additional PVE/awakening/unlock fields. `proto/LoginProfileRes.cs` groups `profile_res`, `skill_res`, and `pet_info`. The V38 profile serializer already emits the common first six fields using the matched Sigma response definition; this source confirms that those are the client’s profile concepts.

The same archive contains `proto/GameOpeningInfoRes.cs`, whose model includes `opening_info_list`, timezone offset, game-mode names, and level-limit lists. Each `GameOpeningInfoDesc` can carry `map_id`, display name, time window, game/match mode, player-count limits, difficulty, weekday, visual-map reference, version, and other flags. `proto/MapConfig.cs` carries an ID, weight, and priority. These are useful response-shape clues, but the archive has no actual catalogue rows mapping the supplied map codes to numeric IDs or downloadable map resources.

`proto/MessageNotify.cs` declares `account_id`, `protocol`, `ret`, `cmd`, and `content`. That describes an envelope’s fields, not how bytes are framed, tagged, encrypted, authenticated, or dispatched on the socket. The decompiled files contain declarations with dummy method bodies rather than the server implementation.

**What this helps:** validating profile data concepts and identifying likely GameOpeningInfo response fields. **What it does not yet enable:** a verified matchmaking TCP exchange, a success/handoff response, map-ID resolution, a game-server prepare handshake, or a real UDP gameplay session. Those still require wire-level framing/serialization details and actual server logic or sanitized client/server traces.
