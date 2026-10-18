package main

import (
	"testing"

	"libmadoka/match-server/packet"
)

func TestNoParamNotifyIsAcceptedWithoutSessionSetup(t *testing.T) {
	s := &session{}
	// RUDP_NOPARAM_NOTIFY is a protocol-defined client notification. It must not be
	// treated as an unknown packet or require a joined match to be present.
	s.route(&packet.Packet{Cmd: packet.CmdNoParamNotify, Payload: []byte("diagnostic-only")})
}
