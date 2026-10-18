package main

import (
	"fmt"
	"log"

	"libmadoka/match-server/message"
	"libmadoka/match-server/packet"
)

// Reconnect-by-account, for clients that re-dial mid-join.
//
// cmd 110 BattleStart is what pulls a pre-1.70 client off the waiting island, but the
// client implements it as LoadMPBattleGame(CREATE_NEW_CONN): it drops the UDP socket,
// opens a NEW one, and posts its join again. Without the handling here that re-join looks
// like a brand-new player, so the manager hands it a fresh roster slot — and then sends it
// cmd 110 again, which makes it re-dial again. That is the loading-screen ↔ map-screen loop:
// each pass burns another slot (1, 2, 3, …) with the same human behind all of them.
//
// The account id from the signed prepare_token is stable across the re-dial, so it is the
// identity to route on: a join whose account is already on a roster RESUMES that player —
// same entity id, same slot, same inventory — on the new connection, and the join tail then
// omits cmd 110 (resumed == true) so the client settles instead of re-dialing forever.

// findByAccount returns the live match holding this account and that player's session, or
// (nil, nil). Called under the manager lock from join().
func (mgr *MatchManager) findByAccount(accountID uint64) (*Match, *session) {
	if accountID == 0 {
		return nil, nil
	}
	for _, h := range mgr.matches {
		m, ok := h.(*Match)
		if !ok || m.ended {
			continue
		}
		for _, p := range m.players {
			if p != nil && p.player.AccountID == accountID {
				return m, p
			}
		}
	}
	return nil, nil
}

// resume moves a player's identity and gameplay state from its old (dead) connection onto
// the new one, then replays the join handshake on the new socket. It runs on the match
// loop's goroutine, so touching m.players here is safe.
//
// What carries over is everything the client cannot tell us again: the entity/RepID the
// rest of the match already knows this player by, the roster slot and team, the inventory
// and equipment, and the last known position. What does NOT carry over is cmd 110 — see
// sendJoinTail — because re-sending it is what caused the re-dial in the first place.
func (m *Match) resume(old, s *session) {
	m.adoptSession(old, s)
	m.sendResumeHandshake(s)
}

// resumeQuiet swaps the new connection in for the old one WITHOUT replaying the join handshake: for a
// cmd 5 transport re-handshake the client is already inside the running battle (it does not re-post
// cmd 440 and must not receive 100/101/130 again). It just re-binds the PRI stream and restarts the
// CS replication loop on the SAME match — so the round, score, HP and loadout all carry on instead of
// restarting at round 1.
func (m *Match) resumeQuiet(old, s *session) {
	m.adoptSession(old, s)
	m.bindAll()
	s.startCSMatch() // no-op when the loop is already running
	s.enqueue(s.resyncWalls)
}

// adoptSession moves identity + gameplay state from the old (dead) connection onto s and swaps s into
// the roster in place of old.
func (m *Match) adoptSession(old, s *session) {
	if s.player.AccountID == 0 {
		s.player = old.player // a bare cmd 5 reconnect carries no token: keep the real player record
	}
	s.entityID, s.repID, s.faction, s.team, s.slot = old.entityID, old.repID, old.faction, old.team, old.slot
	s.player.EntityID = s.entityID
	s.player.TeamIdx = old.player.TeamIdx
	s.player.SpawnPos, s.player.SpawnFace = old.player.SpawnPos, old.player.SpawnFace
	s.playerPos = old.playerPos

	// Carry the loadout so the player does not come back empty-handed.
	s.coins = old.coins
	s.uidCounter = old.uidCounter
	s.equipment = old.equipment
	s.weapons = old.weapons
	s.clientUIDs = old.clientUIDs
	s.itemOnHand = old.itemOnHand
	s.attachments = old.attachments

	s.resumed = true
	s.match = m

	// Swap the new connection into the roster in place of the dead one.
	for i, p := range m.players {
		if p == old {
			m.players[i] = s
			break
		}
	}
	if m.s == old {
		m.s = s
	}
	old.stopped.Store(true)

	log.Printf("[mm-udp] resume: acc=%d ent=%#x slot=%d moved to %v (was %v) — no BattleStart this time",
		s.player.AccountID, s.entityID, s.slot, s.remote, old.remote)
}

// sendResumeHandshake replays the packets the freshly-dialed client needs to rebuild its
// view: the join result, every entity on the roster (its OWN first, so the client adopts it
// as the local player), the join tail, and a re-bind so the PRI stream routes again.
func (m *Match) sendResumeHandshake(s *session) {
	s.sendDataLog(packet.CmdJoinMatchRes,
		message.JoinMatchRes(0, m.settings.roomSetting, m.settings.roomSetting2), "cmd=100 join res (resume)")
	s.sendDataLog(packet.CmdPlayerJoin, message.PlayerJoin(s.player, m.serverTick()),
		fmt.Sprintf("cmd=101 self ent=%#x %q (resume)", s.player.EntityID, s.player.Name))
	for _, p := range m.players {
		if p != s {
			s.sendDataLog(packet.CmdPlayerJoin, message.PlayerJoin(p.player, m.serverTick()),
				fmt.Sprintf("cmd=101 other ent=%#x (resume)", p.player.EntityID))
		}
	}
	m.sendBotJoins(s, "resume")
	s.sendJoinTail("(resume)")
	s.sendDataLog(packet.CmdSyncRegionActivities, message.SyncRegionActivitiesDefault(),
		"cmd=176 SYNC_REGION_ACTIVITIES (resume)")
	m.bindAll()
	if m.arena.City != "" {
		m.broadcastZoneIndex() // cmd 457 — the re-dialed client lost its region index; without it the
	} // round-intro resolves zone 99 and no world region streams in
	for _, p := range m.players { // the pawns render holding their current weapon again
		s.sendDataLog(packet.CmdSyncInventory, p.currentInventorySync(),
			fmt.Sprintf("cmd=174 ent=%#x inventory (resume)", p.entityID))
		p.resendEquipTo(s)
		p.resendAttachTo(s)
	}
	if m.battleRoyale && m.brLootSeeded {
		m.sendBattleRoyaleLootTo(s) // the ground loot already placed before the re-dial
	}
	s.startCSMatch() // no-op when the loop is already running
}

// findForReconnect locates the live match a bare cmd 5 reconnect belongs to. The cmd 5 re-handshake
// carries no signed token, so the account is usually unknown (0); fall back to the roster human on the
// same source IP (the client re-dials from the same device, only the UDP port changes). Without this
// the reconnect minted a brand-new solo match — the "auto-back -> round 1" reset.
func (mgr *MatchManager) findForReconnect(s *session) (*Match, *session) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if m, old := mgr.findByAccount(s.player.AccountID); m != nil {
		return m, old
	}
	if s.remote == nil {
		return nil, nil
	}
	for _, h := range mgr.matches {
		m, ok := h.(*Match)
		if !ok || m.ended || m.battleRoyale || !m.started {
			continue
		}
		for _, p := range m.players {
			if p != nil && p != s && p.remote != nil && p.remote.IP.Equal(s.remote.IP) {
				return m, p
			}
		}
	}
	return nil, nil
}
