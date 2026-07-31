package sandbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func testService(t *testing.T) (*Service, string) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "repo-1")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "binary.bin"), []byte{1, 0, 2}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("metadata"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".stignore"), []byte("sync metadata"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolver, err := NewLayoutResolver(base)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(resolver, Config{MaxReadBytes: 32, MaxEntries: 20})
	if err != nil {
		t.Fatal(err)
	}
	return service, root
}

func TestTreeAndRead(t *testing.T) {
	service, _ := testService(t)
	entries, err := service.Tree(context.Background(), "ws", "repo-1", ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 || entries[0].Kind != "directory" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	for _, entry := range entries {
		if (entry.Name == ".git" || entry.Name == ".stignore") && (!entry.Ignored || entry.Readable) {
			t.Fatalf("metadata entry is exposed: %#v", entry)
		}
	}
	item, err := service.Read(context.Background(), "ws", "repo-1", "src/main.go")
	if err != nil || item.Content != "package main\n" {
		t.Fatalf("read = %#v, err = %v", item, err)
	}
	if _, err := service.Read(context.Background(), "ws", "repo-1", "binary.bin"); !errors.Is(err, ErrBinary) {
		t.Fatalf("binary error = %v", err)
	}
}

func TestPathAndSymlinkGuards(t *testing.T) {
	service, root := testService(t)
	for _, path := range []string{"../secret", "/etc/passwd", `..\secret`, "src/../main.go", "src/./main.go", "src\x00main.go"} {
		if _, err := service.Read(context.Background(), "ws", "repo-1", path); !errors.Is(err, ErrInvalidPath) {
			t.Errorf("path %q error = %v", path, err)
		}
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(filepath.Dir(root), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := service.Read(context.Background(), "ws", "repo-1", "escape"); !errors.Is(err, ErrInvalidPath) && !errors.Is(err, ErrSymlink) {
		t.Fatalf("symlink error = %v", err)
	}
}

func TestReadLimit(t *testing.T) {
	service, root := testService(t)
	if err := os.WriteFile(filepath.Join(root, "large.txt"), make([]byte, 33), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Read(context.Background(), "ws", "repo-1", "large.txt"); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("large file error = %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	service, _ := testService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Tree(ctx, "ws", "repo-1", "."); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
}
