package agentregistry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HostTaskExecutor struct {
	baseURL string
	client  *http.Client
}

func NewHostTaskExecutor(baseURL string) *HostTaskExecutor {
	return &HostTaskExecutor{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 2 * time.Minute},
	}
}

func (executor *HostTaskExecutor) Prompt(
	ctx context.Context, task AgentTask, input string,
) ([]TaskOutput, error) {
	var response struct {
		Outputs []TaskOutput `json:"outputs"`
	}
	err := executor.request(
		ctx,
		"/v1/agents/"+url.PathEscape(task.AgentID)+"/prompt",
		task.OwnerID,
		map[string]string{
			"resource_id": task.ResourceID,
			"thread_id":   task.ThreadID,
			"task_id":     task.ID,
			"prompt":      input,
		},
		&response,
	)
	return response.Outputs, err
}

func (executor *HostTaskExecutor) Cancel(ctx context.Context, task AgentTask) error {
	return executor.request(
		ctx,
		"/v1/agents/"+url.PathEscape(task.AgentID)+"/session/cancel",
		task.OwnerID,
		map[string]string{
			"resource_id": task.ResourceID, "thread_id": task.ThreadID, "task_id": task.ID,
		},
		nil,
	)
}

func (executor *HostTaskExecutor) request(
	ctx context.Context, path, ownerID string, payload, output any,
) error {
	if executor == nil || executor.baseURL == "" {
		return errors.New("owner Host Agent route is not configured")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, executor.baseURL+path, bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-ID", ownerID)
	response, err := executor.client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(response.Body).Decode(&failure)
		return errors.New(fallback(failure.Error, "owner Host Agent rejected the task request"))
	}
	if output == nil {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(output)
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return defaultValue
}
