package main

import (
	"log"
	"sync"
)

// maxPlayers caps a CS match's roster at the user's 8-player limit; the minimum of 2 comes for
// free because a lone human always has the enemy bot for company. The MatchManager enforces the
// cap when routing a joining player (Step 4b wires the routing; until Step 5 there is only ever
// one human per match, so the cap is not yet exercised).
const maxPlayers = 8

// MatchManager owns every live Match in the process and routes a joining player to one: an
// existing match with a free slot, else a fresh match. Step 4b introduces it as the central
// registry (the match list was implicit in main.go's sessions map before); Step 5 uses join()
// to drop a second player into an existing match; Step 7 reaps finished matches from it.
//
// mu guards the match list. It is touched from the receive goroutine at join/registration time,
// NEVER from a match's run() loop (a match never reaches across to another), so it never
// contends with the lock-free single-owner model inside a match.
type MatchManager struct {
	mu      sync.Mutex
	matches []MatchHandle
	factory MatchFactory
}

// matchManager is the process-wide registry. It mints matches through the factory (localFactory ships
// in-process today) and only ever handles them as MatchHandle — the process seam (Step 8, matchhandle.go).
var matchManager = &MatchManager{factory: localFactory{}}

// join routes a joining human to a match — an existing one with a free slot, else a fresh one — and
// admits them. m.reserved (manager-owned under mu) is the race-safe roster-size view; run() owns
// m.players. The first player of a fresh match runs admitFirst inline (which starts run()); a later
// player's admit is enqueued onto the already-running match loop, and its inbound handlers are routed
// to that mailbox from now on (syncStarted).
func (mgr *MatchManager) join(s *session) {
	mgr.mu.Lock()
	// A client that re-dialed (cmd 110 makes pre-1.70 clients open a new socket) posts its
	// join again with the SAME signed account. Route it back to the slot it already holds
	// instead of handing it a new one — see resume.go.
	if m, old := mgr.findByAccount(s.player.AccountID); m != nil && old == s {
		mgr.mu.Unlock() // the SAME connection posted its join twice (cmd 448 + cmd 440/100): it already holds a slot
		log.Printf("[mm-udp] duplicate join post from %v ignored (already in the roster)", s.remote)
		return
	} else if m != nil && old != s {
		mgr.mu.Unlock()
		s.match = m
		s.syncStarted = true
		s.enqueue(func() { m.resume(old, s) })
		return
	}
	var h MatchHandle
	// 1) Same matchmaker match id wins: players the matchmaker put TOGETHER (same `mid` in their signed
	// prepare_token) must land in the SAME match, even if the first player's match has already started
	// (bot fill / buy phase locks canAdmit for strangers, but not for a player who was queued with us).
	if mid := s.player.MatchID; mid != 0 {
		for _, cand := range mgr.matches {
			if mm, ok := cand.(*Match); ok && mm.mid == mid && mm.canAdmitSameID(s) {
				h = cand
				log.Printf("[mm-udp] join route: acc=%d mid=%d -> existing match with the SAME match id (started=%v roster=%d)", s.player.AccountID, mid, mm.started, mm.reserved)
				break
			}
		}
	}
	// 2) Otherwise any match still gathering players with the same map/mode.
	if h == nil {
		for _, cand := range mgr.matches {
			if cand.canAdmit(s) { // an existing match with a free slot, same mode, that hasn't ended
				h = cand
				break
			}
		}
	}
	fresh := h == nil
	if fresh {
		log.Printf("[mm-udp] join route: acc=%d mid=%d -> NEW match (no joinable match found; %d live)", s.player.AccountID, s.player.MatchID, len(mgr.matches))
		h = mgr.factory.create(s) // no room anywhere -> mint a new match (localFactory: in-process)
		mgr.matches = append(mgr.matches, h)
	}
	h.reserve()
	mgr.mu.Unlock()

	h.admit(s, fresh) // wire the player in (fresh starts run(); a later player enqueues onto its loop)
}

// register adds an already-built in-process match to the registry (the reconnect resume in
// handleHello mints its own solo *Match outside join()). It bypasses the factory because a mid-match
// resume is inherently in-process — the client is already connected to THIS process.
func (mgr *MatchManager) register(m *Match) {
	mgr.mu.Lock()
	mgr.matches = append(mgr.matches, m)
	m.reserved++
	mgr.mu.Unlock()
}

// reap removes a finished match from the registry — called from its run() goroutine on exit. It's a
// brief, once-per-match lock, so it doesn't contend with the hot single-owner path inside a match.
func (mgr *MatchManager) reap(m MatchHandle) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	kept := mgr.matches[:0]
	for _, cand := range mgr.matches {
		if cand != m {
			kept = append(kept, cand)
		}
	}
	mgr.matches = kept
}

// count returns the number of live matches (diagnostics / future reaping).
func (mgr *MatchManager) count() int {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return len(mgr.matches)
}
