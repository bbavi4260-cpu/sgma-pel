package main

import (
	"math"
	"testing"
	"time"
)

func TestBRAirplaneRouteEndpoints(t *testing.T) {
	r := brAirplaneRoute
	if got := r.positionAt(0); got != r.Start {
		t.Fatalf("route start = %#v, want %#v", got, r.Start)
	}
	if got := r.positionAt(r.Duration); got != r.End {
		t.Fatalf("route end = %#v, want %#v", got, r.End)
	}
	if got := r.positionAt(r.Duration + time.Second); got != r.End {
		t.Fatalf("route after duration = %#v, want endpoint %#v", got, r.End)
	}
}

func TestBRAirplaneRouteMidpoint(t *testing.T) {
	r := brAirplaneRoute
	got := r.positionAt(r.Duration / 2)
	want := (r.Start.X + r.End.X) / 2
	if math.Abs(got.X-want) > 0.001 {
		t.Fatalf("midpoint X = %f, want %f", got.X, want)
	}
	if got.Y != r.Start.Y || got.Y != r.End.Y {
		t.Fatalf("midpoint Y = %f, route altitude should remain constant", got.Y)
	}
	if r.headingRadians() == 0 {
		t.Fatal("expected non-zero heading for supplied route")
	}
}
