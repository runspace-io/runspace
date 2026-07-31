package filesync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/runspace/runspace/internal/sandbox"
	"github.com/runspace/runspace/internal/workspace"
)

type allowWrites struct{ provider string }

func (allowWrites) CanWrite(context.Context, string, string) error { return nil }
func (a allowWrites) ListRepositories(context.Context, string, string) ([]workspace.Repository, error) {
	provider := a.provider
	if provider == "" {
		provider = "mirror"
	}
	return []workspace.Repository{{ID: "repository-1", Provider: provider}}, nil
}

func TestRegisterPlainFolderDoesNotInitializeGit(t *testing.T) {
	root := t.TempDir()
	resolver, err := sandbox.NewLayoutResolver(root)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(allowWrites{provider: "folder"}, resolver, &fakeEngine{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Register(context.Background(), RegisterRequest{
		UserID: "admin", WorkspaceID: "workspace-1", RepositoryID: "repository-1",
		DeviceID: "host-device", Addresses: []string{"tcp://host.docker.internal:22000"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "repository-1", ".git")); !os.IsNotExist(err) {
		t.Fatalf("plain folder unexpectedly initialized Git: %v", err)
	}
}

type fakeEngine struct {
	device Device
	folder Folder
	ignore []string
}

func (f *fakeEngine) DeviceID(context.Context) (string, error) { return "gateway-device", nil }
func (f *fakeEngine) UpsertDevice(_ context.Context, device Device) error {
	f.device = device
	return nil
}
func (f *fakeEngine) UpsertFolder(_ context.Context, folder Folder) error {
	f.folder = folder
	return nil
}
func (f *fakeEngine) SetIgnores(_ context.Context, _ string, patterns []string) error {
	f.ignore = patterns
	return nil
}
func (f *fakeEngine) Status(context.Context, string) (FolderStatus, error) {
	return FolderStatus{State: "idle"}, nil
}

func TestRegisterConfiguresMirror(t *testing.T) {
	root := t.TempDir()
	resolver, err := sandbox.NewLayoutResolver(root)
	if err != nil {
		t.Fatal(err)
	}
	engine := &fakeEngine{}
	service, err := NewService(allowWrites{}, resolver, engine, []string{"tcp://localhost:22001"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.Register(context.Background(), RegisterRequest{
		UserID:       "admin",
		WorkspaceID:  "workspace-1",
		RepositoryID: "repository-1",
		DeviceID:     "host-device",
		DeviceName:   "Laptop",
		Addresses:    []string{"tcp://host.docker.internal:22000"},
		Git:          true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if session.GatewayDeviceID != "gateway-device" || session.Status.State != "idle" {
		t.Fatalf("session=%+v", session)
	}
	if engine.folder.Path != filepath.Join(root, "repository-1") {
		t.Fatalf("folder path=%q", engine.folder.Path)
	}
	if len(engine.ignore) < 2 || engine.device.ID != "host-device" {
		t.Fatalf("device=%+v ignores=%v", engine.device, engine.ignore)
	}
	if _, err := os.Stat(filepath.Join(root, "repository-1", ".git")); err != nil {
		t.Fatalf("gateway Git metadata was not initialized: %v", err)
	}
}

func TestStatusRecoversSessionAfterServiceRestart(t *testing.T) {
	root := t.TempDir()
	resolver, err := sandbox.NewLayoutResolver(root)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(
		allowWrites{},
		resolver,
		&fakeEngine{},
		[]string{"tcp://localhost:22001"},
	)
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.Status(
		context.Background(),
		"admin",
		"workspace-1",
		"repository-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if session.ID != "workspace-1:repository-1" ||
		session.FolderID != "runspace-workspace-1-repository-1" ||
		session.Status.State != "idle" {
		t.Fatalf("session=%+v", session)
	}
}
