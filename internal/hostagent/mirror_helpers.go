package hostagent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/runspace/runspace/internal/filesync"
	"github.com/runspace/runspace/internal/workspace"
)

type MirrorRequest struct {
	Path        string `json:"path"`
	GatewayURL  string `json:"gateway_url"`
	UserID      string `json:"user_id"`
	WorkspaceID string `json:"workspace_id"`
}

type MirrorResponse struct {
	Resource   workspace.Resource   `json:"resource"`
	Repository workspace.Repository `json:"repository"`
	Sync       filesync.Session     `json:"sync"`
}

func validateRequest(request MirrorRequest) (string, string, error) {
	if strings.TrimSpace(request.UserID) == "" || strings.TrimSpace(request.WorkspaceID) == "" {
		return "", "", errors.New("user and workspace are required")
	}
	path, err := existingDirectory(request.Path)
	if err != nil {
		return "", "", err
	}
	gatewayURL := strings.TrimRight(strings.TrimSpace(request.GatewayURL), "/")
	parsed, err := url.ParseRequestURI(gatewayURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", "", errors.New("valid gateway URL is required")
	}
	return filepath.Clean(path), gatewayURL, nil
}

func existingDirectory(requestedPath string) (string, error) {
	path, err := filepath.Abs(strings.TrimSpace(requestedPath))
	if err != nil {
		return "", errors.New("invalid resource path")
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", errors.New("resource path must be an existing directory")
	}
	return filepath.Clean(path), nil
}

func decodeRequest(writer http.ResponseWriter, request *http.Request, result any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(result); err != nil {
		return errors.New("invalid request")
	}
	return nil
}

func (s *Server) git(ctx context.Context, path string, args ...string) (string, error) {
	commandArgs := append([]string{"-C", path}, args...)
	output, err := exec.CommandContext(ctx, s.gitBinary, commandArgs...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (s *Server) gateway(ctx context.Context, endpoint, userID string, body any, result any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-ID", strings.TrimSpace(userID))
	response, err := s.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		var apiError struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(message, &apiError) == nil && apiError.Error != "" {
			return errors.New(apiError.Error)
		}
		return fmt.Errorf("gateway returned %s", response.Status)
	}
	if result == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(result)
}

func repositoryName(origin, path string) string {
	if strings.TrimSpace(origin) == "" {
		return filepath.Base(path)
	}
	value := strings.TrimSuffix(strings.TrimSpace(origin), ".git")
	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		value = strings.Trim(parsed.Path, "/")
	} else if index := strings.Index(value, ":"); strings.Contains(value[:max(index, 0)], "@") && index >= 0 {
		value = strings.Trim(value[index+1:], "/")
	}
	if value == "" || strings.Contains(value, string(filepath.Separator)) && strings.HasPrefix(value, ".") {
		value = filepath.Base(path)
	}
	return value
}

func (s *Server) localResourceURL(path string) string {
	fingerprint := sha256.Sum256([]byte(s.deviceID + "\x00" + strings.ToLower(filepath.Clean(path))))
	return "local-resource://" + fmt.Sprintf("%x", fingerprint[:12])
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
