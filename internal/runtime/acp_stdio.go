package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// NewStdioACPFactory starts one ACP JSON-RPC peer per agent run. The command
// must speak line-delimited JSON-RPC on stdin/stdout, as defined by ACP.
func NewStdioACPFactory(command string, args ...string) ACPFactory {
	return NewStdioACPFactoryWithOptions(StdioOptions{Command: command, Args: args})
}

type StdioOptions struct {
	Command        string
	Args           []string
	Env            map[string]string
	PermissionMode string
	MCPServers     []MCPServer
}

type MCPServer struct {
	Name    string        `json:"name"`
	Command string        `json:"command"`
	Args    []string      `json:"args"`
	Env     []EnvVariable `json:"env"`
}

type EnvVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func NewStdioACPFactoryWithOptions(options StdioOptions) ACPFactory {
	return func(ctx context.Context) (ACPClient, error) {
		if options.Command == "" {
			return nil, errors.New("ACP command is required")
		}
		process := exec.CommandContext(ctx, options.Command, options.Args...)
		process.Env = append([]string(nil), os.Environ()...)
		for name, value := range options.Env {
			process.Env = append(process.Env, name+"="+value)
		}
		stdin, err := process.StdinPipe()
		if err != nil {
			return nil, err
		}
		stdout, err := process.StdoutPipe()
		if err != nil {
			_ = stdin.Close()
			return nil, err
		}
		if err := process.Start(); err != nil {
			_ = stdin.Close()
			return nil, err
		}
		client := &stdioACP{
			stdin: stdin, pending: make(map[int64]chan rpcResponse),
			notifications:      make(chan ACPNotification, 32),
			configOptions:      make(map[string][]sessionConfigOption),
			pendingPermissions: make(map[string]pendingPermission),
			permissionMode:     options.PermissionMode,
			mcpServers:         cloneMCPServers(options.MCPServers),
		}
		go client.read(stdout)
		return client, nil
	}
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Message string `json:"message"`
}

type stdioACP struct {
	stdin              io.WriteCloser
	mu                 sync.Mutex
	nextID             int64
	questionSeq        int64
	pending            map[int64]chan rpcResponse
	pendingPermissions map[string]pendingPermission
	notifications      chan ACPNotification
	configOptions      map[string][]sessionConfigOption
	permissionMode     string
	mcpServers         []MCPServer
}

type sessionConfigOption struct {
	ID           string `json:"id"`
	Category     string `json:"category"`
	CurrentValue string `json:"currentValue"`
	Options      []struct {
		Value string `json:"value"`
		Name  string `json:"name"`
	} `json:"options"`
}

func (c *stdioACP) request(ctx context.Context, method string, params any, result any) error {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	response := make(chan rpcResponse, 1)
	c.pending[id] = response
	payload, marshalErr := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if marshalErr == nil {
		_, marshalErr = c.stdin.Write(append(payload, '\n'))
	}
	c.mu.Unlock()
	if marshalErr != nil {
		return marshalErr
	}
	select {
	case reply := <-response:
		if reply.Error != nil {
			return errors.New(reply.Error.Message)
		}
		return json.Unmarshal(reply.Result, result)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *stdioACP) Initialize(ctx context.Context) error {
	var result map[string]any
	return c.request(ctx, "initialize", map[string]any{"protocolVersion": 1}, &result)
}

func (c *stdioACP) NewSession(ctx context.Context, cwd string) (string, error) {
	var result struct {
		SessionID     string                `json:"sessionId"`
		ConfigOptions []sessionConfigOption `json:"configOptions"`
	}
	if err := c.request(ctx, "session/new", c.sessionParams(cwd), &result); err != nil {
		return "", err
	}
	if result.SessionID == "" {
		return "", errors.New("ACP returned an empty session ID")
	}
	c.mu.Lock()
	c.configOptions[result.SessionID] = result.ConfigOptions
	c.mu.Unlock()
	return result.SessionID, nil
}

func (c *stdioACP) ResumeSession(ctx context.Context, sessionID, cwd string) error {
	var result struct {
		ConfigOptions []sessionConfigOption `json:"configOptions"`
	}
	params := c.sessionParams(cwd)
	params["sessionId"] = sessionID
	if err := c.request(ctx, "session/resume", params, &result); err != nil {
		return err
	}
	c.mu.Lock()
	c.configOptions[sessionID] = result.ConfigOptions
	c.mu.Unlock()
	return nil
}

func (c *stdioACP) SetSessionModel(ctx context.Context, sessionID, model string) error {
	c.mu.Lock()
	options := append([]sessionConfigOption(nil), c.configOptions[sessionID]...)
	c.mu.Unlock()
	for _, option := range options {
		if option.Category != "model" {
			continue
		}
		for _, candidate := range option.Options {
			if candidate.Value == model || strings.EqualFold(candidate.Name, model) {
				var result struct {
					ConfigOptions []sessionConfigOption `json:"configOptions"`
				}
				err := c.request(ctx, "session/set_config_option", map[string]any{
					"sessionId": sessionID, "configId": option.ID, "value": candidate.Value,
				}, &result)
				if err == nil {
					c.mu.Lock()
					c.configOptions[sessionID] = result.ConfigOptions
					c.mu.Unlock()
				}
				return err
			}
		}
		return fmt.Errorf("model %q is not offered by this agent", model)
	}
	return errors.New("agent does not advertise model selection")
}

func (c *stdioACP) Prompt(ctx context.Context, sessionID, prompt string) error {
	content := []map[string]string{{"type": "text", "text": prompt}}
	var result map[string]any
	return c.request(ctx, "session/prompt", map[string]any{"sessionId": sessionID, "prompt": content}, &result)
}

func (c *stdioACP) Cancel(ctx context.Context, sessionID string) error {
	var result map[string]any
	return c.request(ctx, "session/cancel", map[string]any{"sessionId": sessionID}, &result)
}

func (c *stdioACP) Notifications() <-chan ACPNotification { return c.notifications }

func (c *stdioACP) Close() error {
	c.cancelPendingPermissions()
	return c.stdin.Close()
}

func (c *stdioACP) read(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var message struct {
			ID     *int64          `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *rpcError       `json:"error"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			continue
		}
		if message.ID != nil {
			if message.Method == "session/request_permission" {
				c.respondPermission(*message.ID, scanner.Bytes())
				continue
			}
			c.mu.Lock()
			pending := c.pending[*message.ID]
			delete(c.pending, *message.ID)
			c.mu.Unlock()
			if pending != nil {
				pending <- rpcResponse{Result: message.Result, Error: message.Error}
			}
			continue
		}
		if message.Method == "session/update" {
			c.notify(message.Params)
		}
	}
	close(c.notifications)
}

// notify forwards one session/update to consumers. The raw params ride along so
// callers keep structured detail that the flattened text drops. The send never
// blocks: a stalled consumer must not wedge the ACP reader loop.
func (c *stdioACP) notify(params json.RawMessage) {
	var update struct {
		SessionID string `json:"sessionId"`
		Update    struct {
			Kind    string `json:"sessionUpdate"`
			Content struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"update"`
	}
	if json.Unmarshal(params, &update) != nil || update.SessionID == "" {
		return
	}
	select {
	case c.notifications <- ACPNotification{
		SessionID: update.SessionID,
		Kind:      update.Update.Kind,
		Text:      update.Update.Content.Text,
		Payload:   append(json.RawMessage(nil), params...),
	}:
	default:
	}
}

var _ ACPClient = (*stdioACP)(nil)

func (e rpcError) Error() string { return fmt.Sprintf("ACP error: %s", e.Message) }
