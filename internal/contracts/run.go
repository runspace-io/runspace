package contracts

import "fmt"

type RunState string

const (
	RunQueued    RunState = "queued"
	RunRunning   RunState = "running"
	RunStopping  RunState = "stopping"
	RunSucceeded RunState = "succeeded"
	RunFailed    RunState = "failed"
	RunCancelled RunState = "cancelled"
)

func TransitionRun(from, to RunState) error {
	if from == to {
		return nil
	}
	allowed := map[RunState]map[RunState]bool{
		RunQueued:   {RunRunning: true, RunCancelled: true},
		RunRunning:  {RunStopping: true, RunSucceeded: true, RunFailed: true, RunCancelled: true},
		RunStopping: {RunCancelled: true, RunFailed: true},
	}
	if allowed[from][to] {
		return nil
	}
	return fmt.Errorf("invalid run transition %q -> %q", from, to)
}
