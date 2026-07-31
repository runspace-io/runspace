package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

type CommandRunner interface {
	Run(context.Context, string, ...string) (string, error)
}
type DockerExecutor struct {
	runner CommandRunner
	image  string
}

func NewDockerExecutor(r CommandRunner, image string) DockerExecutor {
	if image == "" {
		image = "runspace-agent:latest"
	}
	return DockerExecutor{runner: r, image: image}
}
func SafeRunWorkspace(root, runID string) (string, error) {
	if strings.TrimSpace(runID) == "" || strings.ContainsAny(runID, `/\\`) || runID == "." || runID == ".." {
		return "", errors.New("invalid run ID")
	}
	p := filepath.Join(root, runID)
	clean, _ := filepath.Abs(p)
	base, _ := filepath.Abs(root)
	if clean == base || !strings.HasPrefix(clean, base+string(filepath.Separator)) {
		return "", errors.New("workspace escapes root")
	}
	return clean, nil
}
func (d DockerExecutor) Run(ctx context.Context, workspace, runID string) error {
	if d.runner == nil {
		return errors.New("runner is nil")
	}
	if workspace == "" || runID == "" {
		return errors.New("workspace and run ID required")
	}
	safeWorkspace, err := SafeRunWorkspace(workspace, runID)
	if err != nil {
		return err
	}
	args := []string{"run", "--rm", "--network", "none", "--cpus", "2", "--memory", "4g", "--pids-limit", "256", "--user", "1000:1000", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "-v", safeWorkspace + ":/workspace:rw", "-w", "/workspace", d.image, "sh", "-lc", "codex"}
	_, err = d.runner.Run(ctx, "", args...)
	return err
}
