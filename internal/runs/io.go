package runs

import (
	"context"
	"errors"

	"github.com/runspace/runspace/internal/contracts"
)

func (s *Service) Input(ctx context.Context, id, text string) error {
	if text == "" {
		return errors.New("input is required")
	}
	run, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if run.Status != contracts.RunRunning {
		return errors.New("run is not running")
	}
	inputAgent, ok := s.agentFor(id).(contracts.InputAgent)
	if !ok {
		return errors.New("agent does not accept input")
	}
	if err := inputAgent.Send(ctx, contracts.InputRequest{RunID: id, Text: text}); err != nil {
		return err
	}
	s.publish(ctx, "run.input", run)
	return nil
}
func (s *Service) agentFor(id string) contracts.Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if agent, ok := s.runAgents[id]; ok {
		return agent
	}
	return s.agent
}
func (s *Service) publish(ctx context.Context, eventType string, run Run) {
	if s.publisher == nil {
		return
	}
	event, err := contracts.NewEvent(run.ID+"-"+eventType, eventType, run.WorkspaceID, "system", "system", run, run.UpdatedAt)
	if err == nil {
		_ = s.publisher.Publish(ctx, event)
	}
}
