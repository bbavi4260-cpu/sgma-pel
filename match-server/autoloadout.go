package main

import (
	"log"
	"os"
	"time"

	"libmadoka/match-server/message"
	"libmadoka/match-server/packet"
)

// Auto-loadout: a FREE starter kit granted at the start of every buy phase, so a player always has real
// guns + armour even when the client's shop UI shows nothing (the shop list is client-version sensitive).
// It only fills EMPTY slots, so a player who survived a round keeps what they own. Uses the exact same
// delivery path as a shop buy (deliverPurchasedItem), just without charging coins or the popup.
// MATCH_AUTO_LOADOUT=0 turns it off.

// autoLoadoutOrder: primary1, primary2, vest, helmet, gloo wall, frag grenade.
func (m *Match) grantAutoLoadout() {
	if os.Getenv("MATCH_AUTO_LOADOUT") == "0" || m.battleRoyale {
		return
	}
	for _, p := range m.players {
		p.grantAutoLoadout()
	}
}

func (s *session) grantAutoLoadout() {
	m := s.match
	grant := func(id uint32, qty uint32) {
		it, ok := shopItemByIDIn(csShopItems, id)
		if !ok {
			return
		}
		cp := *it
		s.deliverPurchasedItem(&cp, qty, false)
	}
	_, havePrimary1 := s.weapons[message.SlotPrimary1]
	_, havePrimary2 := s.weapons[message.SlotPrimary2]
	if !havePrimary1 {
		grant(6, 1) // AK
	}
	if !havePrimary2 {
		grant(2, 1) // M4A1
	}
	if s.vestData == 0 {
		grant(303, 1) // lvl3 vest
	}
	if s.helmetData == 0 {
		grant(306, 1) // lvl3 helmet
	}
	if _, n := s.glooCount(); n == 0 {
		grant(1201, 3) // gloo wall ("blue ball")
	}
	if _, _, ok := s.trackedByData(601, 0); !ok {
		grant(601, 1) // frag grenade
	}
	s.sendVar(packet.CmdPRISync, m.priPayload(), 1)
	log.Printf("[mm-udp] auto-loadout granted to %#x (AK, M4A1, vest, helmet, gloo, grenade where empty)", s.entityID)
}

// stepShopResend re-sends the shop catalogue every few seconds while the buy phase or a fight is on, so the
// store opens populated whenever the player taps BUY (the client keeps the list only for a short window).
func (m *Match) stepShopResend(now time.Time) {
	if os.Getenv("MATCH_SHOP_RESEND") != "1" || m.battleRoyale || (m.phase != phPrepare && m.phase != phFight) { // opt-in: extra cmd 413s can reach the client mid-load
		return
	}
	if now.Sub(m.lastShopSend) < 4*time.Second {
		return
	}
	m.lastShopSend = now
	for _, p := range m.players {
		p.sendCSShop()
	}
}
