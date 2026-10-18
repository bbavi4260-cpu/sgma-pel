package main

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"log"
	"os"

	"libmadoka/match-server/message"
	"libmadoka/match-server/packet"
)

// route dispatches one decoded packet to its handler. The ack + the serve()/run() routing are
// done by dispatch; route runs either inline (pre-match) or on run()'s goroutine (via the mailbox).
// Each case is a thin delegate — the actual work lives in the handleXxx methods below.
func (s *session) route(p *packet.Packet) {
	switch p.Cmd {
	case packet.CmdHello, packet.CmdReconnect: // 1 / 5
		s.handleHello(p)
	case packet.CmdAck: // 2 — client's ACK of our reliable packets; stub does not resend.
	case packet.CmdPing: // 3
		s.handlePing(p)
	case packet.CmdJoinMatchPrepare, packet.CmdJoinMatchPrepareLegacy: // 447 current; 439 legacy
		s.handleJoinMatchPrepare(p)
	case packet.CmdJoinMatchPostLegacy: // 440 legacy join-post dialect
		s.handleJoinMatchPost(p)
	case packet.CmdJoinMatchPost: // 448 current RUDP_JOIN_MATCH_POST
		s.legacyJoin = os.Getenv("MATCH_ALT_JOIN_LEGACY") == "1"
		log.Printf("[mm-udp] join posted on cmd=448 (current protocol, legacyTail=%v) %v", s.legacyJoin, s.remote)
		s.handleJoinMatchPost(p)
	// Some client builds post the join on cmd 100 instead of 440. 100 is OUR
	// JoinMatchRes opcode, so it is only ever RECEIVED from a client that means
	// "join this match" — the server never reads back what it sends, which makes
	// this alias unambiguous. Verified against a live capture: the payload is the
	// same JoinMatchReq layout (UserID, RoomID = match id, then the length-prefixed
	// prepare_token), and prepareToken finds the JWT by its header marker anyway, so
	// no offset assumptions are needed. Without this the join is logged as
	// "UNHANDLED cmd=100" and the player sits on the waiting island forever.
	case packet.CmdJoinMatchRes: // 100 — legacy client-sent join alias
		// Posting the join here instead of on 440 identifies a pre-1.70 client, which also
		// needs the legacy join TAIL (empty cmd 130 + cmd 110 BattleStart) — see sendJoinTail.
		s.legacyJoin = true
		log.Printf("[mm-udp] join posted on cmd=100 (pre-1.70 client dialect) -> legacy join tail %v", s.remote)
		s.handleJoinMatchPost(p)
	case packet.CmdCSPurchase, packet.CmdCSPurchaseLegacy: // 414 current / 408 legacy
		s.purchaseCmd = p.Cmd // reply on the same ID the client used
		s.handleCSPurchase(p)
	case packet.CmdAddIcewall, packet.CmdIcewallFire: // 218 (333 alias) — client's gloo-wall PLACE request (bidirectional cmd)
		s.handlePlaceIcewall(p)
	case packet.CmdIcewallTakeDamage: // 219 — a placed gloo wall took damage
		s.handleIcewallDamage(p)
	case packet.CmdTakeDamage: // 106
		s.handleTakeDamage(p)
	case packet.CmdWeaponAction: // 104 — start-fire / weapon-lift: relay + set the fire-state
		s.handleWeaponAction(p)
	case packet.CmdStopFire: // 105 — stop firing
		s.handleStopFire(p)
	case packet.CmdReloadStart, packet.CmdReloadFinish: // 133 / 134 — relay the reload animation
		s.handleReloadRelay(p)
	case packet.CmdProjectileThrow, packet.CmdProjectileExplode, packet.CmdStartGrenade, packet.CmdStopGrenade: // 160-163 — relay grenade throw/explode/windup
		s.handleProjectileRelay(p)
	case packet.CmdStartSniper: // 137 — scope open: set the ADS pose (PRI field 12) + relay
		s.handleStartSniper(p)
	case packet.CmdStopSniper: // 138 — scope close
		s.handleStopSniper(p)
	case packet.CmdChangeHeldItem: // 108
		s.handleChangeHeldItem(p)
	case packet.CmdEquip: // 119 — inventory slot swap (drag an item to another loadout slot)
		s.handleEquip(p)
	case packet.CmdPickupInventory: // 111 — pick up a ground-loot item
		s.handlePickup(p)
	case packet.CmdDropInventory: // 112 — drop a loadout weapon to the ground
		s.handleDrop(p)
	case packet.CmdUseInventory: // 113 — finished using a consumable (medkit heal-over-time)
		s.handleUseInventory(p)
	case packet.CmdTryUseInventory: // 131 — started channelling a consumable
		s.handleTryUseInventory(p)
	case packet.CmdEmote: // 369 — play emote: relay the animation to others
		s.handleEmote(p)
	case packet.CmdBattleFlagReq: // 276 — client planted a flag-emote flag: spawn/replace the named world object
		s.handleBattleFlagReq(p)
	case packet.CmdEquipAttachment: // 122 — client's manual attachment equip (server auto-maxes)
		s.handleEquipAttachment(p)
	case packet.CmdUnequipAttachment: // 123 — never unequip; attachments stay locked on
		s.handleUnequipAttachment(p)
	case packet.CmdDeviceReport: // 480 — device/integrity report from a pre-1.70 client
		s.handleDeviceReport(p)
	case packet.CmdClientReady, packet.CmdClientLoaded: // 190 / 340 — pre-1.70 ready/loaded signals
		s.handleClientReady(p)
	case packet.CmdClientPos: // 1001
		s.handleClientPos(p)
	case packet.CmdPlayerQuitReq: // 191
		s.handlePlayerQuit(p)
	case packet.CmdStartRescue: // 142 — teammate began reviving a knocked player
		s.handleStartRescue(p)
	case packet.CmdStopRescue: // 143 — teammate stopped reviving
		s.handleStopRescue(p)
	default:
		s.handleUnknown(p)
	}
}

// handleHello answers HELLO / reconnect with S2C_Hello_Res. A cmd 5 reconnect is a
// mid-match transport re-handshake: the client is already in the running battle and
// does NOT re-post cmd 440, so the join packets (100/101/130) must NOT be resent —
// but our CS replication stream died with the old session, so resume it.
func (s *session) handleHello(p *packet.Packet) {
	res := packet.BuildHelloRes("libmadoka", 0, 0, false)
	s.sendHelloRes(res)
	log.Printf("[mm-udp] HELLO(cmd=%d) %v -> S2C_Hello_Res (%dB)", p.Cmd, s.remote, len(res))
	if p.Cmd == packet.CmdReconnect && s.match == nil {
		if m, old := matchManager.findForReconnect(s); m != nil && old != s {
			s.match, s.syncStarted, s.joined = m, true, true
			s.mode = old.mode
			s.enqueue(func() { m.resumeQuiet(old, s) }) // same match -> same round/score, not a fresh round 1
			log.Printf("[mm-udp] reconnect -> resumed existing match round=%d score=%d-%d %v", m.round, m.teamScore[0], m.teamScore[1], s.remote)
			return
		}
		if s.player.AccountID == 0 {
			s.player, s.mode = resolvePlayer("") // fallback player (EntityID=1, RepID 1000) — matches the original join
		}
		s.joined = true
		m := newMatch(s)
		matchManager.register(m)
		m.setupMatch(s) // arena / roster / round / bot — no re-join packets (client is already in-match)
		log.Printf("[mm-udp] reconnect mid-match -> solo resume (no re-join) %v", s.remote)
		s.startCSMatch()
		s.enqueue(s.resyncWalls) // redraw lost gloo walls on run()'s goroutine (cmd 220)
	}
}

// handlePing echoes the ping back unreliably to keep the client's ping timer alive. The reply goes
// straight onto the socket (an unreliable packet carries no per-connection seq/order, so bypassing the
// Writer is safe) instead of waiting in the Writer's FIFO behind a burst of PRI/GRI/ack traffic —
// queueing delay there is counted by the client as ping.
func (s *session) handlePing(p *packet.Packet) {
	pkt := &packet.Packet{SendOption: packet.SendUnreliable, Cmd: packet.CmdPing, Flags: 0, Payload: p.Payload}
	wire, err := pkt.Encode(s.key)
	if err != nil || s.conn == nil || s.remote == nil {
		s.write(pkt) // fall back to the queued path
		return
	}
	if _, err := s.conn.WriteToUDP(wire, s.remote); err != nil {
		log.Printf("[mm-udp] ping echo write err: %v", err)
	}
}

// handleJoinMatchPost handles the join-post command for the selected protocol dialect:
// cmd 448 in the current enum, or cmd 440 in the legacy dialect. It decodes the prepare_token, resolves the player,
// pick a spawn, answer the join (cmd 100/101/130), and start the CS state stream.
//
// NOTE: cmd 110 BattleStart is NOT used for Contra Squad — it triggers
// LoadMPBattleGame(CREATE_NEW_CONN) (leave waiting island), which just makes the CS
// client reconnect without unlocking movement. The GRI (cmd 901) is the movement
// driver; the authoritative spawn is the position in cmd 101 (see choosePlayerSpawn).
func (s *session) handleJoinMatchPost(p *packet.Packet) {
	tok := s.prepareToken(p.Payload)
	// Consume the prepare fragment exactly once so a reconnect/retry cannot
	// accidentally append a previous join's token to a later request.
	s.prep439 = nil
	pl, mode, verified := resolvePlayerChecked(tok)
	s.player, s.mode = pl, mode
	if !verified {
		// Bad/missing prepare_token: every such client would otherwise share the SAME fallback account id, and
		// the match manager would treat the 2nd one as a reconnect of the 1st (findByAccount) — they'd never
		// see each other. Give each connection its own stable id derived from its UDP address.
		f := fnv.New32a()
		f.Write([]byte(s.remote.String()))
		s.player.AccountID = fallbackAccountID + 1 + uint64(f.Sum32()%900000)
		s.player.Name = fmt.Sprintf("%s%d", fallbackName, s.player.AccountID%1000)
		log.Printf("[mm-udp] WARNING prepare_token missing/invalid -> temporary unique identity acc=%d %q (check MATCH_JWT_SECRET is IDENTICAL for the matchmaker and the UDP node) %v",
			s.player.AccountID, s.player.Name, s.remote)
	}
	s.joined = true
	matchManager.join(s) // Step 5b: route to a match (existing with room, else new) + admit
}

// handleJoinMatchPrepare handles cmd 447 in the current enum (or 439 in the legacy dialect): the client sends the
// prepare_token (JWT) here as [u32 totalLen][token chunk] when it is large. A small token
// skips 439 and rides entirely in cmd 440; a token too big for one packet is split, the
// bigger half here and the tail in cmd 440. We buffer this chunk (and its declared total
// length); handleJoinMatchPost reassembles the full token.
func (s *session) handleJoinMatchPrepare(p *packet.Packet) {
	if len(p.Payload) < 4 {
		return
	}
	chunkLen := binary.LittleEndian.Uint32(p.Payload[0:])
	chunk := p.Payload[4:]
	if chunkLen > 0 && int(chunkLen) < len(chunk) { // never keep more than declared
		chunk = chunk[:chunkLen]
	}
	s.prep439 = append([]byte(nil), chunk...)
	log.Printf("[mm-udp] JOIN_MATCH_PREPARE cmd=%d token chunk=%dB (declared %d) %v", p.Cmd, len(s.prep439), chunkLen, s.remote)
}

// prepareToken reassembles the prepare_token (JWT) from the buffered prepare chunk and
// the join-post payload. It can arrive wholly in the post payload (small token), wholly in prepare,
// or split across 439 (bigger chunk) + 440 (tail). The 1.70 cmd 440 struct offsets don't
// line up, so scanning for the fixed JWT header marker is the reliable path.
func (s *session) prepareToken(payload440 []byte) string {
	if len(s.prep439) > 0 {
		tok := message.ExtractJWT(s.prep439) // chunk starts at the JWT header
		if tok == "" {
			tok = string(s.prep439) // no marker (raw token bytes) — take as-is
		}
		// The 439 chunk is the bigger half; when it isn't already a whole JWT the tail (the
		// rest of the signature) rides in cmd 440 as the last length-prefixed base64url
		// field, after the metadata. Append it to complete the token.
		if !message.IsCompleteJWT(tok) {
			tail := message.LastJWTField(payload440)
			tok += tail
			log.Printf("[mm-udp] JOIN prepare_token split: 439(%dB)+440 tail(%dB) -> %dB %v",
				len(s.prep439), len(tail), len(tok), s.remote)
		} else {
			log.Printf("[mm-udp] JOIN prepare_token whole in cmd 439 (%dB) %v", len(tok), s.remote)
		}
		return tok
	}
	if tok := message.ExtractJWT(payload440); tok != "" {
		log.Printf("[mm-udp] JOIN prepare_token found in cmd 440 by scan (%dB) %v", len(tok), s.remote)
		return tok
	}
	if req, err := message.ParseJoinMatchReq(payload440); err == nil && req.Token != "" {
		log.Printf("[mm-udp] JOIN prepare_token from cmd 440 struct field (%dB) %v", len(req.Token), s.remote)
		return req.Token
	}
	log.Printf("[mm-udp] JOIN no prepare_token; cmd440(%dB)=%x %v", len(payload440), payload440, s.remote)
	return ""
}

// The cmd-1001 movement state. The client's upstream is id(4) + a 36-byte CORE (pos(12)+moveDir(6)+
// targetDir(4)+velocity(6)+moveState(1)+flag(1)+aimDir(6)); a platform byte may or may not follow.
// The downstream relay entry is [u8 slot] ++ a 37-byte body (the core + a platform byte). See
// cs-movement-sync.
const (
	moveCoreLen = 36
	moveBodyLen = 37
)

// handleClientPos parses the client's per-tick position update (cmd 1001): it stores the local
// player's world position for the SafeZone out-of-zone check AND the full state body for the
// movement relay (Step 6). Payload leads with [id u32][X i32][Y i32][Z i32] (coords ×1000 mm),
// then dirs/velocity/state — 45 bytes on foot.
func (s *session) handleClientPos(p *packet.Packet) {
	if len(p.Payload) < 16 {
		return
	}
	x := int32(binary.LittleEndian.Uint32(p.Payload[4:]))
	y := int32(binary.LittleEndian.Uint32(p.Payload[8:]))
	z := int32(binary.LittleEndian.Uint32(p.Payload[12:]))
	pos := message.Vec3{X: float64(x) / 1000, Y: float64(y) / 1000, Z: float64(z) / 1000}
	s.playerPos = pos
	// Keep the state body for the relay batch. Upstream = id(4) + 36-byte core (pos..aim); the
	// platform byte may or may not follow. Take the core from [4:40] and the platform from [40] if
	// present (else 0); the downstream entry is [slot] ++ this 37-byte body.
	if len(p.Payload) >= 4+moveCoreLen {
		if len(s.moveBody) != moveBodyLen {
			s.moveBody = make([]byte, moveBodyLen)
		}
		copy(s.moveBody, p.Payload[4:4+moveCoreLen])
		if len(p.Payload) > 4+moveCoreLen {
			s.moveBody[moveCoreLen] = p.Payload[4+moveCoreLen] // platform byte, if the client sent it
		} else {
			s.moveBody[moveCoreLen] = 0
		}
	}
	// The client's own send-seq rides in the wrapper's trailing u32 [41:45]; the FIRST one anchors
	// the relay seq (moveBase) — large enough to clear the client's per-pawn seq gate.
	if len(p.Payload) >= 45 && s.moveBase == 0 {
		s.moveBase = binary.LittleEndian.Uint32(p.Payload[41:45])
	}
	if cfg.debugPos {
		log.Printf("[mm-udp] POS cmd=1001 -> (%.1f,%.1f,%.1f)", pos.X, pos.Y, pos.Z)
	}
	// The first cmd 1001 is the earliest packet guaranteed to be AFTER the client's OnSceneLoaded (its
	// pawn must exist to report position), i.e. inside the replay recording window. Re-emit the cmd-101
	// roster ONCE so client replays spawn pawns + dismiss the loading mask. No-op on the live client
	// (idempotent). Runs on run()'s mailbox (cmd 1001 is enqueued once syncStarted). See reseedJoinBurst.
	if s.match != nil {
		s.match.reseedJoinBurst()
	}
	// LIVE sync: relay this state to every other player RIGHT NOW (event-driven, ~client send rate) instead of
	// waiting for the next relay tick. Only once the match loop owns the state (this handler then runs on it).
	s.moveDirty = true
	if s.syncStarted && s.match != nil {
		s.match.relayMove(s)
	}
}

// handlePlayerQuit handles cmd 191 (RUDP_PLAYER_QUIT_REQ): acknowledge with the quit
// result (cmd 192) and stop the CS sync loop so the server stops streaming GRI/PRI to
// a client that has left the match.
func (s *session) handlePlayerQuit(p *packet.Packet) {
	// Auto-back: after the enemy kills you the client pops back out and posts a quit. That is NOT a real leave
	// while the CS match is running: the round just ended and the loser must come back for the next round.
	// Replying with cmd 192 (PLAYER_QUIT_RES) is what makes the client actually LEAVE to the lobby (the loser
	// saw "Tie" and dropped out while the winner played on alone), so a quit from a down player — or any quit
	// during the between-rounds transition — is IGNORED: no cmd 192, roster slot kept, the round-reset revive
	// (cmd 394) brings the player back. A deliberate leave still works once the player is alive again or the
	// match has ended. sweepDisconnects drops the slot if the client truly stops sending.
	if m := s.match; m != nil && !m.battleRoyale && m.started && !m.ended && (m.lifeOf(s.entityID) != lifeAlive || m.inRoundTransition()) {
		log.Printf("[mm-udp] quit while down/between rounds ent=%#x -> IGNORED, no cmd 192 (round %d phase=%d life=%v)", s.entityID, m.round, m.phase, m.lifeOf(s.entityID))
		return
	}
	s.stopped.Store(true)
	s.sendDataLog(packet.CmdPlayerQuitRes, message.PlayerQuit(1), fmt.Sprintf("cmd=192 PlayerQuit payload=%x", p.Payload))
	s.match.clearFlag(s.entityID) // despawn the quitter's flag (it never auto-removes client-side)
	s.match.removePlayer(s)       // drop from the roster; the match keeps running for the others
}

// handleStartRescue handles cmd 142 (RUDP_START_RESCURE): a teammate began reviving a knocked player.
// Payload (LE): [target u32][startMs u64].
func (s *session) handleStartRescue(p *packet.Packet) {
	if len(p.Payload) < 12 {
		return
	}
	target := binary.LittleEndian.Uint32(p.Payload[0:])
	startMs := binary.LittleEndian.Uint64(p.Payload[4:])
	s.match.startRescue(s.entityID, target, startMs)
}

// handleStopRescue handles cmd 143 (RUDP_STOP_RESCURE): the teammate stopped reviving. Payload: [target u32].
func (s *session) handleStopRescue(p *packet.Packet) {
	if len(p.Payload) < 4 {
		return
	}
	target := binary.LittleEndian.Uint32(p.Payload[0:])
	s.match.stopRescue(s.entityID, target)
}

// handleUnknown logs an unrecognised packet with its full payload for RE.
func (s *session) handleUnknown(p *packet.Packet) {
	if p.Cmd == packet.CmdSyncServerTime {
		return
	}
	log.Printf("[mm-udp] %v UNHANDLED cmd=%d so=%d seq=%d order=%d flags=%d len=%d payload=%x",
		s.remote, p.Cmd, p.SendOption, p.SeqID, p.OrderID, p.Flags, len(p.Payload), p.Payload)
}

// handleDeviceReport answers cmd 480, the device/integrity report a pre-1.70 client sends
// once after joining. Its payload is a length-prefixed fingerprint string plus a few
// trailing fields; what the client expects BACK is unknown, so the reply shape is a knob:
//
//	MATCH_480_REPLY=echo  (default) send the same payload back — the usual shape for an
//	                      ack in this protocol family, where the client matches the report
//	                      it sent against the one it gets
//	MATCH_480_REPLY=empty send cmd 480 with no payload
//	MATCH_480_REPLY=none  stay silent (the behaviour before this handler existed)
//
// This is the last client message the server left unanswered. If the client is holding its
// pawn back until this is acknowledged, one of these shapes releases it; if none does, the
// gate is elsewhere and the knob costs nothing.
func (s *session) handleDeviceReport(p *packet.Packet) {
	mode := os.Getenv("MATCH_480_REPLY")
	if mode == "" {
		mode = "echo"
	}
	preview := p.Payload
	if len(preview) > 48 {
		preview = preview[:48]
	}
	log.Printf("[mm-udp] cmd=480 device report (%dB, reply=%s) head=%x %v", len(p.Payload), mode, preview, s.remote)
	switch mode {
	case "none":
		return
	case "empty":
		s.sendDataLog(packet.CmdDeviceReport, nil, "cmd=480 reply (empty)")
	default:
		s.sendDataLog(packet.CmdDeviceReport, append([]byte(nil), p.Payload...), "cmd=480 reply (echo)")
	}
}

// handleClientReady answers cmd 190 / cmd 340, the ready/loaded signals a pre-1.70 client
// sends once it is inside the match. The 1.70 path never receives them, so they were falling
// through to handleUnknown and going unanswered — and an unanswered handshake step is exactly
// what kept this client on the waiting island until cmd 480 was answered.
//
// The reply shape is the same knob as cmd 480 (MATCH_READY_REPLY=echo|empty|none, default
// echo), because what the client wants back is not documented anywhere available here.
func (s *session) handleClientReady(p *packet.Packet) {
	mode := os.Getenv("MATCH_READY_REPLY")
	if mode == "" {
		mode = "echo"
	}
	log.Printf("[mm-udp] cmd=%d client ready/loaded (%dB payload=%x, reply=%s) %v", p.Cmd, len(p.Payload), p.Payload, mode, s.remote)
	switch mode {
	case "none":
		return
	case "empty":
		s.sendDataLog(p.Cmd, nil, fmt.Sprintf("cmd=%d ready reply (empty)", p.Cmd))
	default:
		s.sendDataLog(p.Cmd, append([]byte(nil), p.Payload...), fmt.Sprintf("cmd=%d ready reply (echo)", p.Cmd))
	}
}
