package hostagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/runspace/runspace/internal/filesync"
)

type recordingEngine struct {
	device filesync.Device
	folder filesync.Folder
	ignore []string
}

func (e *recordingEngine) DeviceID(context.Context) (string, error) { return "host-device", nil }
func (e *recordingEngine) UpsertDevice(_ context.Context, device filesync.Device) error {
	e.device = device
	return nil
}
func (e *recordingEngine) UpsertFolder(_ context.Context, folder filesync.Folder) error {
	e.folder = folder
	return nil
}
func (e *recordingEngine) SetIgnores(_ context.Context, _ string, patterns []string) error {
	e.ignore = patterns
	return nil
}
func (*recordingEngine) Status(context.Context, string) (filesync.FolderStatus, error) {
	return filesync.FolderStatus{}, nil
}

func TestCreateMirrorPairsBothDevices(t *testing.T) {
	path := t.TempDir()
	runGit(t, path, "init", "-b", "main")
	runGit(t, path, "remote", "add", "origin", "https://github.com/runspace/demo.git")

	var calls int
	gateway := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		writer.Header().Set("Content-Type", "application/json")
		switch calls {
		case 1:
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"id": "repo-1", "provider": "mirror", "full_name": "runspace/demo",
				"clone_url": "https://github.com/runspace/demo.git", "default_branch": "main",
			})
		case 2:
			_ = json.NewEncoder(writer).Encode(map[string]string{"path": "/repositories/repo-1"})
		case 3:
			_ = json.NewEncoder(writer).Encode(filesync.Session{
				FolderID: "runspace-workspace-1-repo-1", GatewayDeviceID: "gateway-device",
				GatewayAddresses: []string{"tcp://127.0.0.1:22001"},
			})
		default:
			t.Fatalf("unexpected request %s", request.URL.Path)
		}
	}))
	defer gateway.Close()

	engine := &recordingEngine{}
	server, err := NewServer(engine)
	if err != nil {
		t.Fatal(err)
	}
	result, err := server.CreateMirror(context.Background(), MirrorRequest{
		Path: path, GatewayURL: gateway.URL, UserID: "admin", WorkspaceID: "workspace-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Repository.ID != "repo-1" || calls != 3 {
		t.Fatalf("result=%+v calls=%d", result, calls)
	}
	if engine.device.ID != "gateway-device" || engine.folder.Path != filepath.Clean(path) {
		t.Fatalf("device=%+v folder=%+v", engine.device, engine.folder)
	}
	if len(engine.ignore) < 2 {
		t.Fatalf("ignores=%v", engine.ignore)
	}
}

func TestCreateMirrorDoesNotRequireOrigin(t *testing.T) {
	path := t.TempDir()
	runGit(t, path, "init", "-b", "main")

	var paths []string
	gateway := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.URL.Path)
		writer.Header().Set("Content-Type", "application/json")
		if len(paths) == 1 {
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"id": "repo-local", "provider": "mirror", "full_name": filepath.Base(path),
				"clone_url": "local-mirror://" + filepath.Base(path), "default_branch": "main",
			})
			return
		}
		_ = json.NewEncoder(writer).Encode(filesync.Session{
			FolderID: "local-folder", GatewayDeviceID: "gateway-device",
			GatewayAddresses: []string{"tcp://127.0.0.1:22001"},
		})
	}))
	defer gateway.Close()

	engine := &recordingEngine{}
	server, err := NewServer(engine)
	if err != nil {
		t.Fatal(err)
	}
	result, err := server.CreateMirror(context.Background(), MirrorRequest{
		Path: path, GatewayURL: gateway.URL, UserID: "admin", WorkspaceID: "workspace-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Repository.ID != "repo-local" || len(paths) != 2 {
		t.Fatalf("result=%+v paths=%v", result, paths)
	}
	for _, requestPath := range paths {
		if filepath.Base(requestPath) == "clone" {
			t.Fatalf("local-only mirror unexpectedly called clone: %v", paths)
		}
	}
}

func TestCreateMirrorAcceptsPlainFolder(t *testing.T) {
	path := t.TempDir()
	var calls int
	gateway := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		calls++
		writer.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"id": "folder-1", "provider": "folder", "full_name": filepath.Base(path),
				"clone_url": "local-mirror://" + filepath.Base(path), "default_branch": "",
			})
			return
		}
		_ = json.NewEncoder(writer).Encode(filesync.Session{
			FolderID: "folder-1", GatewayDeviceID: "gateway-device",
			GatewayAddresses: []string{"tcp://127.0.0.1:22001"},
		})
	}))
	defer gateway.Close()

	server, err := NewServer(&recordingEngine{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := server.CreateMirror(context.Background(), MirrorRequest{
		Path: path, GatewayURL: gateway.URL, UserID: "admin", WorkspaceID: "workspace-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Repository.Provider != "folder" || calls != 2 {
		t.Fatalf("result=%+v calls=%d", result, calls)
	}
}

func TestInspectRepositoryTreatsNestedFolderAsPlainFolder(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o750); err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(&recordingEngine{})
	if err != nil {
		t.Fatal(err)
	}
	status, err := server.InspectRepository(context.Background(), nested)
	if err != nil {
		t.Fatal(err)
	}
	if status.Git || status.Path != filepath.Clean(nested) || !status.CanConnect {
		t.Fatalf("status=%+v", status)
	}
}

func runGit(t *testing.T, path string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", path}, args...)
	if output, err := exec.Command("git", commandArgs...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, output, err)
	}
}
