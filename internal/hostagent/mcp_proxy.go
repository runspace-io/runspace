package hostagent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxMCPMessageBytes = 4 << 20

func RunMCPProxy(
	ctx context.Context, input io.Reader, output io.Writer,
	endpoint, userID string, client *http.Client,
) error {
	endpoint = strings.TrimSpace(endpoint)
	userID = strings.TrimSpace(userID)
	if endpoint == "" || userID == "" {
		return errors.New("MCP proxy URL and user are required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), maxMCPMessageBytes)
	for scanner.Scan() {
		message := append([]byte(nil), scanner.Bytes()...)
		if len(bytes.TrimSpace(message)) == 0 {
			continue
		}
		response, notification, err := relayMCPMessage(ctx, client, endpoint, userID, message)
		if err != nil {
			return err
		}
		if notification {
			continue
		}
		if _, err := output.Write(append(response, '\n')); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func relayMCPMessage(
	ctx context.Context, client *http.Client, endpoint, userID string, message []byte,
) ([]byte, bool, error) {
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
	}
	if json.Unmarshal(message, &envelope) != nil ||
		envelope.JSONRPC != "2.0" || strings.TrimSpace(envelope.Method) == "" {
		return nil, false, errors.New("invalid MCP JSON-RPC message")
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint, bytes.NewReader(message),
	)
	if err != nil {
		return nil, false, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("X-User-ID", userID)
	response, err := client.Do(request)
	if err != nil {
		return nil, false, err
	}
	defer response.Body.Close()
	notification := len(envelope.ID) == 0 || string(envelope.ID) == "null"
	body, err := io.ReadAll(io.LimitReader(response.Body, maxMCPMessageBytes))
	if err != nil {
		return nil, notification, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, notification, fmt.Errorf("Runspace MCP returned %s", response.Status)
	}
	if notification {
		return nil, true, nil
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, false, errors.New("Runspace MCP returned an empty response")
	}
	return body, false, nil
}
