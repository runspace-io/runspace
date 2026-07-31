package hostagent

import (
	"context"
	"strings"
	"time"
)

func (s *Server) RunPresence(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	s.publishPresence(ctx)
	for {
		select {
		case <-ticker.C:
			s.publishPresence(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Server) publishPresence(ctx context.Context) {
	agents := s.DiscoverAgents()
	s.mu.RLock()
	targets := make(map[string]map[string]struct{})
	for userID, user := range s.config.Users {
		for _, resource := range user.Resources {
			if strings.TrimSpace(resource.GatewayURL) == "" {
				continue
			}
			if targets[userID] == nil {
				targets[userID] = make(map[string]struct{})
			}
			targets[userID][strings.TrimRight(resource.GatewayURL, "/")] = struct{}{}
		}
		for _, resource := range user.Capabilities {
			if strings.TrimSpace(resource.GatewayURL) == "" {
				continue
			}
			if targets[userID] == nil {
				targets[userID] = make(map[string]struct{})
			}
			targets[userID][strings.TrimRight(resource.GatewayURL, "/")] = struct{}{}
		}
	}
	s.mu.RUnlock()
	for userID, gateways := range targets {
		for gatewayURL := range gateways {
			_ = s.gateway(
				ctx, gatewayURL+"/users/me/agents/presence", userID,
				map[string]any{"agents": agents}, nil,
			)
		}
	}
}
