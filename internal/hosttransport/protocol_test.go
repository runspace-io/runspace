package hosttransport

import (
	"context"
	"testing"
)

type fakeTransport struct{ route Route }

func (transport fakeTransport) Route() Route { return transport.route }
func (transport fakeTransport) Available(state PeerState) bool {
	switch transport.route {
	case RouteLoopback:
		return state.SameDevice
	case RouteDirect:
		return state.DirectReachable
	default:
		return state.RelayConnected
	}
}
func (transport fakeTransport) Call(context.Context, Envelope) (Response, error) {
	return Response{OK: true}, nil
}

func TestSelectPrefersLoopbackThenDirectThenRelay(t *testing.T) {
	transports := []Transport{
		fakeTransport{route: RouteRelay},
		fakeTransport{route: RouteDirect},
		fakeTransport{route: RouteLoopback},
	}
	selected, err := Select(PeerState{SameDevice: true, DirectReachable: true, RelayConnected: true}, transports...)
	if err != nil || selected.Route() != RouteLoopback {
		t.Fatalf("selected=%v err=%v", selected, err)
	}
	selected, err = Select(PeerState{DirectReachable: true, RelayConnected: true}, transports...)
	if err != nil || selected.Route() != RouteDirect {
		t.Fatalf("selected=%v err=%v", selected, err)
	}
}
