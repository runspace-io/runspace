package filesync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Device struct {
	ID        string   `json:"deviceID"`
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
}

type Folder struct {
	ID               string         `json:"id"`
	Label            string         `json:"label"`
	Path             string         `json:"path"`
	Type             string         `json:"type"`
	FSWatcherEnabled bool           `json:"fsWatcherEnabled"`
	FSWatcherDelayS  int            `json:"fsWatcherDelayS"`
	IgnorePerms      bool           `json:"ignorePerms"`
	Devices          []FolderDevice `json:"devices"`
}

type FolderDevice struct {
	DeviceID string `json:"deviceID"`
}

type FolderStatus struct {
	State      string `json:"state"`
	Error      string `json:"error"`
	LocalFiles int    `json:"localFiles"`
	NeedFiles  int    `json:"needFiles"`
	NeedBytes  int64  `json:"needBytes"`
}

type Engine interface {
	DeviceID(context.Context) (string, error)
	UpsertDevice(context.Context, Device) error
	UpsertFolder(context.Context, Folder) error
	SetIgnores(context.Context, string, []string) error
	Status(context.Context, string) (FolderStatus, error)
}

type SyncthingClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewSyncthingClient(baseURL, apiKey string) (*SyncthingClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("syncthing URL and API key are required")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("parse syncthing URL: %w", err)
	}
	return &SyncthingClient{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(apiKey),
		client:  &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (c *SyncthingClient) DeviceID(ctx context.Context) (string, error) {
	var result struct {
		MyID string `json:"myID"`
	}
	if err := c.request(ctx, http.MethodGet, "/rest/system/status", nil, &result); err != nil {
		return "", err
	}
	if result.MyID == "" {
		return "", errors.New("syncthing returned an empty device ID")
	}
	return result.MyID, nil
}

func (c *SyncthingClient) UpsertDevice(ctx context.Context, device Device) error {
	if strings.TrimSpace(device.ID) == "" {
		return errors.New("device ID is required")
	}
	return c.request(ctx, http.MethodPost, "/rest/config/devices", device, nil)
}

func (c *SyncthingClient) UpsertFolder(ctx context.Context, folder Folder) error {
	if strings.TrimSpace(folder.ID) == "" || strings.TrimSpace(folder.Path) == "" {
		return errors.New("folder ID and path are required")
	}
	return c.request(ctx, http.MethodPost, "/rest/config/folders", folder, nil)
}

func (c *SyncthingClient) SetIgnores(ctx context.Context, folderID string, patterns []string) error {
	path := "/rest/db/ignores?folder=" + url.QueryEscape(folderID)
	return c.request(ctx, http.MethodPost, path, map[string]any{"ignore": patterns}, nil)
}

func (c *SyncthingClient) Status(ctx context.Context, folderID string) (FolderStatus, error) {
	var status FolderStatus
	path := "/rest/db/status?folder=" + url.QueryEscape(folderID)
	return status, c.request(ctx, http.MethodGet, path, nil, &status)
}

func (c *SyncthingClient) request(
	ctx context.Context,
	method string,
	path string,
	body any,
	result any,
) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode syncthing request: %w", err)
		}
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("create syncthing request: %w", err)
	}
	request.Header.Set("X-API-Key", c.apiKey)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("call syncthing: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("syncthing returned %s: %s", response.Status, strings.TrimSpace(string(message)))
	}
	if result == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(result); err != nil {
		return fmt.Errorf("decode syncthing response: %w", err)
	}
	return nil
}
