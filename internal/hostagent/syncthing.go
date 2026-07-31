package hostagent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/runspace/runspace/internal/filesync"
)

type SyncthingRuntime struct {
	Client  *filesync.SyncthingClient
	Command *exec.Cmd
}

func StartSyncthing(ctx context.Context) (*SyncthingRuntime, error) {
	baseURL := fallback(strings.TrimSpace(os.Getenv("RUNSPACE_SYNCTHING_URL")), "http://127.0.0.1:8385")
	stateDir, err := syncStateDir()
	if err != nil {
		return nil, err
	}
	apiKey, err := loadOrCreateAPIKey(stateDir)
	if err != nil {
		return nil, err
	}
	client, err := filesync.NewSyncthingClient(baseURL, apiKey)
	if err != nil {
		return nil, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	_, probeErr := client.DeviceID(probeCtx)
	cancel()
	if probeErr == nil {
		return &SyncthingRuntime{Client: client}, nil
	}

	binary := fallback(strings.TrimSpace(os.Getenv("SYNCTHING_BIN")), "syncthing")
	command := exec.CommandContext(ctx, binary,
		"serve",
		"--no-browser",
		"--no-upgrade",
		"--home="+stateDir,
		"--gui-address=127.0.0.1:8385",
		"--gui-apikey="+apiKey,
	)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start Syncthing (%s): %w", binary, err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		probeCtx, probeCancel := context.WithTimeout(ctx, time.Second)
		_, err = client.DeviceID(probeCtx)
		probeCancel()
		if err == nil {
			return &SyncthingRuntime{Client: client, Command: command}, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	_ = command.Process.Kill()
	return nil, errors.New("timed out waiting for Syncthing to become ready within 20 seconds")
}

func syncStateDir() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("RUNSPACE_HOST_STATE_DIR")); configured != "" {
		if err := os.MkdirAll(configured, 0o700); err != nil {
			return "", err
		}
		return filepath.Abs(configured)
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	path := filepath.Join(configDir, "Runspace", "syncthing")
	if err := os.MkdirAll(path, 0o700); err != nil {
		return "", fmt.Errorf("create sync state directory: %w", err)
	}
	return path, nil
}

func loadOrCreateAPIKey(stateDir string) (string, error) {
	path := filepath.Join(stateDir, "api-key")
	if value, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(value)) != "" {
		return strings.TrimSpace(string(value)), nil
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate sync API key: %w", err)
	}
	value := hex.EncodeToString(random)
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		return "", fmt.Errorf("save sync API key: %w", err)
	}
	return value, nil
}
