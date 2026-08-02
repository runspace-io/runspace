package collaboration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runspace/runspace/internal/auth"
	"github.com/runspace/runspace/internal/workspace"
)

func TestHTTPChatRoutesAreAuthorized(t *testing.T) {
	workspaceService := workspace.NewMemoryService(nil)
	workspaceModel, err := workspaceService.CreateWorkspace(context.Background(), "alice", workspace.CreateWorkspaceRequest{Name: "Alpha"})
	if err != nil {
		t.Fatal(err)
	}
	service := NewMemoryService(nil, workspaceService)
	router := NewHandler(service).Routes()
	request := httptest.NewRequest(http.MethodPost, "/workspaces/"+workspaceModel.ID+"/threads", strings.NewReader(`{"title":"Ship it"}`))
	request = request.WithContext(auth.WithUserID(request.Context(), "alice"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var created Thread
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	messagesRequest := httptest.NewRequest(http.MethodGet, "/threads/"+created.ID+"/messages?workspace_id="+workspaceModel.ID, nil)
	messagesRequest = messagesRequest.WithContext(auth.WithUserID(messagesRequest.Context(), "alice"))
	messagesResponse := httptest.NewRecorder()
	router.ServeHTTP(messagesResponse, messagesRequest)
	if messagesResponse.Code != http.StatusOK || !strings.Contains(messagesResponse.Body.String(), `"messages":[]`) {
		t.Fatalf("empty messages status=%d body=%s", messagesResponse.Code, messagesResponse.Body.String())
	}
	unauthorized := httptest.NewRequest(http.MethodGet, "/workspaces/"+workspaceModel.ID+"/threads", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, unauthorized)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", response.Code)
	}
}

func TestHTTPThreadCanBeBoundToChannel(t *testing.T) {
	workspaceService := workspace.NewMemoryService(nil)
	workspaceModel, err := workspaceService.CreateWorkspace(context.Background(), "alice", workspace.CreateWorkspaceRequest{Name: "Alpha"})
	if err != nil {
		t.Fatal(err)
	}
	service := NewMemoryService(nil, workspaceService)
	channel, err := service.CreateChannel(context.Background(), "alice", workspaceModel.ID, "backend", "", "repo-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	router := NewHandler(service).Routes()
	request := httptest.NewRequest(http.MethodPost, "/workspaces/"+workspaceModel.ID+"/threads", strings.NewReader(`{"title":"Backend chat","channel_id":"`+channel.ID+`"}`))
	request = request.WithContext(auth.WithUserID(request.Context(), "alice"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"channel_id":"`+channel.ID+`"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestHTTPChannelPatch(t *testing.T) {
	workspaceService := workspace.NewMemoryService(nil)
	workspaceModel, err := workspaceService.CreateWorkspace(context.Background(), "alice", workspace.CreateWorkspaceRequest{Name: "Alpha"})
	if err != nil {
		t.Fatal(err)
	}
	service := NewMemoryService(nil, workspaceService)
	channel, err := service.CreateChannel(context.Background(), "alice", workspaceModel.ID, "backend", "", "repo-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	router := NewHandler(service).Routes()
	request := httptest.NewRequest(http.MethodPatch, "/workspaces/"+workspaceModel.ID+"/channels/"+channel.ID, strings.NewReader(`{"name":"api","config":{"agent":"acp"}}`))
	request = request.WithContext(auth.WithUserID(request.Context(), "alice"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"name":"api"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestHTTPChannelUsesResourceContract(t *testing.T) {
	workspaceService := workspace.NewMemoryService(nil)
	workspaceModel, err := workspaceService.CreateWorkspace(context.Background(), "alice", workspace.CreateWorkspaceRequest{Name: "Alpha"})
	if err != nil {
		t.Fatal(err)
	}
	router := NewHandler(NewMemoryService(nil, workspaceService)).Routes()
	request := httptest.NewRequest(
		http.MethodPost,
		"/workspaces/"+workspaceModel.ID+"/channels",
		strings.NewReader(`{"name":"api","resource_ids":["resource-1","resource-2"]}`),
	)
	request = request.WithContext(auth.WithUserID(request.Context(), "alice"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusCreated ||
		!strings.Contains(body, `"resource_ids":["resource-1","resource-2"]`) ||
		!strings.Contains(body, `"repository_ids":["resource-1","resource-2"]`) {
		t.Fatalf("status=%d body=%s", response.Code, body)
	}
}
