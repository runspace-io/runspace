package agentregistry

import (
	"context"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/collaboration"
	"github.com/runspace/runspace/internal/workspace"
)

func TestInstallationsAreScopedToUser(t *testing.T) {
	service := New(func() time.Time { return time.Unix(10, 0) })
	item := Installation{
		ID: "local_agent_abc", RegistryID: "opencode", Name: "OpenCode",
		Protocol: "acp", Placement: "host", Status: "ready",
	}
	if _, err := service.Upsert(context.Background(), "nahid", item); err != nil {
		t.Fatal(err)
	}
	other, err := service.List(context.Background(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 0 {
		t.Fatal("local installation leaked to another user")
	}
}

type registryAuthorizer struct{}

func (registryAuthorizer) CanRead(context.Context, string, string) error { return nil }
func (registryAuthorizer) ListMembers(context.Context, string, string) ([]workspace.Member, error) {
	return []workspace.Member{
		{UserID: "admin", Role: workspace.RoleOwner},
		{UserID: "nahid", Role: workspace.RoleMember},
	}, nil
}

type registryMessageWriter struct{ activity string }

func (*registryMessageWriter) CreateAgentMessage(
	context.Context, string, string, string, string, string,
) (collaboration.Message, error) {
	return collaboration.Message{}, nil
}

type registryTaskExecutor struct {
	task  AgentTask
	input string
}

func (executor *registryTaskExecutor) Prompt(
	_ context.Context, task AgentTask, input string,
) ([]TaskOutput, error) {
	executor.task, executor.input = task, input
	return []TaskOutput{{Kind: "text", Text: "Implemented the titled work"}}, nil
}

func (*registryTaskExecutor) Cancel(context.Context, AgentTask) error { return nil }

func (writer *registryMessageWriter) CreateAgentActivity(
	_ context.Context, _, agentID, _, threadID, body string,
) (collaboration.Message, error) {
	writer.activity = body
	return collaboration.Message{
		ID: "message-1", ThreadID: threadID, ActorID: agentID,
		ActorType: "activity", Body: body,
	}, nil
}

func TestTaskGrantDerivesCapabilitiesAndRequiresWorkspaceMember(t *testing.T) {
	service := New(func() time.Time { return time.Unix(10, 0) }, registryAuthorizer{})
	item := Installation{
		ID: "local_agent_abc", RegistryID: "codex-acp", Name: "Codex",
		Protocol: "acp", Placement: "host", Status: "ready",
	}
	if _, err := service.Upsert(context.Background(), "admin", item); err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpsertTask(context.Background(), "admin", AgentTask{
		ID: "local_session_1", WorkspaceID: "workspace-1", ThreadID: "thread-1",
		AgentID: item.ID, ResourceID: "resource-1", Title: "Implement task permissions",
	}); err != nil {
		t.Fatal(err)
	}
	grant, err := service.GrantTaskAccess(context.Background(), "admin", TaskGrant{
		TaskID: "local_session_1", WorkspaceID: "workspace-1",
		AgentID: item.ID, PrincipalID: "nahid", Role: "operator",
		Permissions: []string{"filesystem.admin"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if grant.OwnerID != "admin" || grant.PrincipalID != "nahid" {
		t.Fatalf("unexpected grant identity: %#v", grant)
	}
	if len(grant.Permissions) != 5 || grant.Permissions[2] != "task.control" {
		t.Fatalf("permissions must be server-derived: %#v", grant.Permissions)
	}
	if _, err := service.GrantTaskAccess(context.Background(), "admin", TaskGrant{
		TaskID: "local_session_1", WorkspaceID: "workspace-1",
		AgentID: item.ID, PrincipalID: "outsider", Role: "viewer",
	}); err == nil {
		t.Fatal("non-member received task access")
	}
}

func TestActivityCopyIsServerGenerated(t *testing.T) {
	service := New(func() time.Time { return time.Unix(10, 0) })
	item := Installation{
		ID: "local_agent_abc", RegistryID: "codex-acp", Name: "Codex",
		Protocol: "acp", Placement: "host", Status: "ready",
	}
	if _, err := service.Upsert(context.Background(), "admin", item); err != nil {
		t.Fatal(err)
	}
	writer := &registryMessageWriter{}
	service.SetMessageWriter(writer)
	message, err := service.RecordActivity(
		context.Background(), "admin", "workspace-1", "thread-1",
		item.ID, ActivityCompleted,
	)
	if err != nil {
		t.Fatal(err)
	}
	if message.ActorType != "activity" || writer.activity !=
		"Agent chat work completed. Results remain private until shared." {
		t.Fatalf("unsafe or unexpected activity: %#v", message)
	}
}

func TestContributorCanContinueOwnersTitledTask(t *testing.T) {
	service := New(func() time.Time { return time.Unix(10, 0) }, registryAuthorizer{})
	executor := &registryTaskExecutor{}
	writer := &registryMessageWriter{}
	service.SetTaskExecutor(executor)
	service.SetMessageWriter(writer)
	agent := Installation{
		ID: "local_agent_abc", RegistryID: "codex-acp", Name: "Codex",
		Protocol: "acp", Placement: "host", Status: "ready",
	}
	if _, err := service.Upsert(context.Background(), "admin", agent); err != nil {
		t.Fatal(err)
	}
	task, err := service.UpsertTask(context.Background(), "admin", AgentTask{
		ID: "local_session_fix_terminal", WorkspaceID: "workspace-1", ThreadID: "thread-1",
		AgentID: agent.ID, ResourceID: "resource-1", Title: "Fix invisible terminal input",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.GrantTaskAccess(context.Background(), "admin", TaskGrant{
		TaskID: task.ID, WorkspaceID: task.WorkspaceID, AgentID: agent.ID,
		PrincipalID: "nahid", Role: "contributor",
	}); err != nil {
		t.Fatal(err)
	}
	outputs, err := service.InputTask(
		context.Background(), "nahid", task.ID, "Verify the terminal echo fix",
	)
	if err != nil {
		t.Fatal(err)
	}
	if executor.task.OwnerID != "admin" || executor.input != "Verify the terminal echo fix" {
		t.Fatalf("collaborator input was not routed to the owner task: %#v", executor)
	}
	if len(outputs) != 1 || outputs[0].Text != "Implemented the titled work" {
		t.Fatalf("unexpected private task outputs: %#v", outputs)
	}
	if writer.activity != "Agent chat work completed. Results remain private until shared." {
		t.Fatalf("task completion was not projected safely: %q", writer.activity)
	}
}
