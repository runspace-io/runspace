package terminal

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type processFactory func(context.Context, string, ...string) process

type DockerFactory struct {
	image      string
	volume     string
	newProcess processFactory
}

func NewDockerFactory(image string) *DockerFactory {
	return NewDockerFactoryWithVolume(image, "")
}

func NewDockerFactoryWithVolume(image, volume string) *DockerFactory {
	if strings.TrimSpace(image) == "" {
		image = "runspace-agent:latest"
	}
	return &DockerFactory{image: image, volume: strings.TrimSpace(volume), newProcess: newDockerProcess}
}

func newDockerProcess(ctx context.Context, name string, args ...string) process {
	command := exec.CommandContext(ctx, name, args...)
	return execProcess{command: command}
}

type execProcess struct{ command *exec.Cmd }

func (p execProcess) Start() error { return p.command.Start() }
func (p execProcess) Wait() error  { return p.command.Wait() }
func (p execProcess) Kill() error {
	if p.command.Process == nil {
		return nil
	}
	return p.command.Process.Kill()
}
func (p execProcess) StdinPipe() (io.WriteCloser, error) { return p.command.StdinPipe() }
func (p execProcess) StdoutPipe() (io.ReadCloser, error) { return p.command.StdoutPipe() }
func (p execProcess) StderrPipe() (io.ReadCloser, error) { return p.command.StderrPipe() }

func (f *DockerFactory) Open(ctx context.Context, request OpenRequest) (Session, error) {
	if f == nil || f.newProcess == nil || !validRequest(request) {
		return nil, ErrInvalidRequest
	}
	root, err := safeRoot(request.Root)
	if err != nil {
		return nil, err
	}
	args := dockerArgsWithVolume(f.image, f.volume, root, request.Command)
	return newProcessSession(ctx, f.newProcess(ctx, "docker", args...))
}

func validRequest(request OpenRequest) bool {
	command := strings.TrimSpace(request.Command)
	return request.WorkspaceID != "" && request.RepositoryID != "" && command != "" && len(command) <= maxCommandBytes && !strings.ContainsRune(command, '\x00')
}

func safeRoot(root string) (string, error) {
	if !filepath.IsAbs(root) || strings.ContainsRune(root, '\x00') {
		return "", ErrInvalidRequest
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		return "", ErrInvalidRequest
	}
	return filepath.Clean(canonical), nil
}

func dockerArgs(image, root, command string) []string {
	return dockerArgsWithVolume(image, "", root, command)
}

func dockerArgsWithVolume(image, volume, root, command string) []string {
	mount := []string{"-v", root + ":/workspace:rw", "-w", "/workspace"}
	if volume != "" {
		mount = []string{"-v", volume + ":/var/lib/runspace/repositories:rw", "-w", root}
	}
	return append([]string{"run", "--rm", "-i", "--network", "none", "--cpus", "2", "--memory", "4g", "--pids-limit", "256", "--user", "1000:1000", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--tmpfs", "/tmp:rw,noexec,nosuid,size=64m"}, append(mount, image, "sh", "-lc", command)...)
}

var _ Factory = (*DockerFactory)(nil)
