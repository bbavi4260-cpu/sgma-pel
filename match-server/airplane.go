package main

import (
	"math"
	"time"

	"libmadoka/match-server/message"
)

// brAirplaneRoute is the server-side BR flight path. Coordinates are the route
// supplied for this map; they are kept separate from brSpawnPoints (landing/loot).
var brAirplaneRoute = airplaneRoute{
	Start:    message.Vec3{X: 1352, Y: 347, Z: 513},
	End:      message.Vec3{X: 193, Y: 347, Z: -129},
	Duration: 90 * time.Second,
}

type airplaneRoute struct {
	Start    message.Vec3
	End      message.Vec3
	Duration time.Duration
}

// positionAt returns the linear route position at elapsed time, clamped to the
// route endpoints. This is server-side route math only; it does not itself create
// a visible client aircraft or define an unverified network packet.
func (r airplaneRoute) positionAt(elapsed time.Duration) message.Vec3 {
	if r.Duration <= 0 || elapsed >= r.Duration {
		return r.End
	}
	if elapsed <= 0 {
		return r.Start
	}
	t := float64(elapsed) / float64(r.Duration)
	return message.Vec3{
		X: r.Start.X + (r.End.X-r.Start.X)*t,
		Y: r.Start.Y + (r.End.Y-r.Start.Y)*t,
		Z: r.Start.Z + (r.End.Z-r.Start.Z)*t,
	}
}

// headingRadians returns the horizontal yaw from Start toward End.
func (r airplaneRoute) headingRadians() float64 {
	return math.Atan2(r.End.X-r.Start.X, r.End.Z-r.Start.Z)
}
