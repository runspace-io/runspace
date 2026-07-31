// Package hosttransport defines the transport-neutral RPC contract between
// Runspace coordination services and user-owned Host Agents.
package hosttransport

import (
	"context"
	"errors"
	"time"
)

type Route string

const (
	RouteLoopback Route = "loopback"
	RouteDirect   Route = "direct"
	RouteRelay    Route = "relay"
)

type Method string

const (
	MethodAgentPrompt         Method = "agent.prompt"
	MethodAgentSessionSummary Method = "agent.session.summary"
	MethodAgentSessionHealth  Method = "agent.session.health"
	MethodResourceRead        Method = "resource.read"
	MethodResourceTree        Method = "resource.tree"
	MethodResourceQuery       Method = "resource.query"
	MethodTerminalOpen        Method = "terminal.open"
	MethodHealthProbe         Method = "host.health"
)

type Envelope struct {
	ID             string         `json:"id"`
	Method         Method         `json:"method"`
	CallerUserID   string         `json:"caller_user_id"`
	OwnerUserID    string         `json:"owner_user_id"`
	WorkspaceID    string         `json:"workspace_id"`
	TargetID       string         `json:"target_id"`
	Capability     string         `json:"capability"`
	GrantID        string         `json:"grant_id"`
	IdempotencyKey string         `json:"idempotency_key"`
	Deadline       time.Time      `json:"deadline"`
	Payload        map[string]any `json:"payload,omitempty"`
}

type Response struct {
	ID      string         `json:"id"`
	OK      bool           `json:"ok"`
	Payload map[string]any `json:"payload,omitempty"`
	Error   *RPCError      `json:"error,omitempty"`
}

type RPCError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type Transport interface {
	Route() Route
	Available(PeerState) bool
	Call(context.Context, Envelope) (Response, error)
}

type PeerState struct {
	SameDevice      bool
	DirectReachable bool
	RelayConnected  bool
}

var ErrNoRoute = errors.New("no host transport route is available")

// Select prefers the cheapest route without changing RPC semantics.
func Select(state PeerState, transports ...Transport) (Transport, error) {
	order := []Route{RouteLoopback, RouteDirect, RouteRelay}
	for _, route := range order {
		for _, transport := range transports {
			if transport.Route() == route && transport.Available(state) {
				return transport, nil
			}
		}
	}
	return nil, ErrNoRoute
}
