package resourcegraph

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

	"github.com/go-chi/chi/v5"
)

type CapabilityQuery struct {
	Capability string `json:"capability"`
	Query      string `json:"query"`
	Limit      int    `json:"limit"`
}

type CapabilityMatch struct {
	Title     string         `json:"title"`
	Summary   string         `json:"summary,omitempty"`
	Reference string         `json:"reference,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type CapabilityQuerier interface {
	Query(context.Context, string, string, string, CapabilityQuery) (any, error)
	Availability(context.Context, string, string, string) (any, error)
}

type PlacementCapabilityQuerier struct {
	placements map[string]CapabilityQuerier
}

func NewPlacementCapabilityQuerier(
	placements map[string]CapabilityQuerier,
) *PlacementCapabilityQuerier {
	return &PlacementCapabilityQuerier{placements: placements}
}

func (q *PlacementCapabilityQuerier) Query(
	ctx context.Context, ownerID, resourceID, placement string, input CapabilityQuery,
) (any, error) {
	querier := q.placements[placement]
	if querier == nil {
		return nil, errors.New("resource placement is unavailable")
	}
	return querier.Query(ctx, ownerID, resourceID, placement, input)
}

func (q *PlacementCapabilityQuerier) Availability(
	ctx context.Context, ownerID, resourceID, placement string,
) (any, error) {
	querier := q.placements[placement]
	if querier == nil {
		return nil, errors.New("resource placement is unavailable")
	}
	return querier.Availability(ctx, ownerID, resourceID, placement)
}

type HostCapabilityQuerier struct {
	baseURL string
	client  *http.Client
}

func NewHostCapabilityQuerier(baseURL string) *HostCapabilityQuerier {
	return &HostCapabilityQuerier{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 20 * time.Second},
	}
}

func (q *HostCapabilityQuerier) Query(
	ctx context.Context, ownerID, resourceID, _ string, input CapabilityQuery,
) (any, error) {
	body, _ := json.Marshal(input)
	target := q.baseURL + "/v1/capability-resources/" + url.PathEscape(resourceID) + "/query"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-ID", ownerID)
	return q.do(request)
}

func (q *HostCapabilityQuerier) Availability(
	ctx context.Context, ownerID, resourceID, _ string,
) (any, error) {
	target := q.baseURL + "/v1/capability-resources/" + url.PathEscape(resourceID) + "/availability"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-User-ID", ownerID)
	return q.do(request)
}

func (q *HostCapabilityQuerier) do(request *http.Request) (any, error) {
	response, err := q.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("resource owner host is unavailable: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 300 {
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(payload, &failure)
		return nil, errors.New(defaultString(failure.Error, "resource request failed"))
	}
	var result any
	if json.Unmarshal(payload, &result) != nil {
		return nil, errors.New("resource host returned invalid output")
	}
	return result, nil
}

func (h *Handler) SetCapabilityQuerier(querier CapabilityQuerier) {
	h.mcp.capabilities = querier
}

func (h *Handler) queryCapability(writer http.ResponseWriter, request *http.Request) {
	var input CapabilityQuery
	if err := decodeGraphBody(request, &input); err != nil {
		writeGraphError(writer, err)
		return
	}
	result, err := h.mcp.queryCapability(
		request, graphUserID(request), chi.URLParam(request, "workspaceID"),
		graphNodeID(request), input,
	)
	if err != nil {
		writeGraphError(writer, err)
		return
	}
	writeGraphJSON(writer, http.StatusOK, result)
}

func (h *Handler) capabilityAvailability(writer http.ResponseWriter, request *http.Request) {
	graphContext, resourceID, err := h.resourceTarget(request)
	if err != nil {
		writeGraphError(writer, err)
		return
	}
	result, err := h.mcp.capabilities.Availability(
		request.Context(), graphContext.Node.OwnerID, resourceID,
		metadataString(graphContext.Node.Metadata, "placement"),
	)
	if err != nil {
		writeGraphJSON(writer, http.StatusOK, map[string]any{
			"resource_id": resourceID, "status": "unavailable",
			"reason": "Resource owner host is offline or unreachable.",
		})
		return
	}
	writeGraphJSON(writer, http.StatusOK, result)
}

func (h *MCPHandler) queryCapability(
	request *http.Request, userID, workspaceID, nodeID string, input CapabilityQuery,
) (any, error) {
	if h.capabilities == nil {
		return nil, errors.New("resource query transport is unavailable")
	}
	graphContext, err := h.service.GetContext(request.Context(), userID, workspaceID, nodeID)
	if err != nil {
		return nil, err
	}
	resourceID, _ := graphContext.Node.Metadata["entity_id"].(string)
	if graphContext.Node.Kind != KindResource || resourceID == "" || input.Capability == "" {
		return nil, ErrInvalid
	}
	return h.capabilities.Query(
		request.Context(), graphContext.Node.OwnerID, resourceID,
		metadataString(graphContext.Node.Metadata, "placement"), input,
	)
}

func (h *Handler) resourceTarget(request *http.Request) (Context, string, error) {
	if h.mcp.capabilities == nil {
		return Context{}, "", errors.New("resource query transport is unavailable")
	}
	graphContext, err := h.service.GetContext(
		request.Context(), graphUserID(request), chi.URLParam(request, "workspaceID"),
		graphNodeID(request),
	)
	if err != nil {
		return Context{}, "", err
	}
	resourceID, _ := graphContext.Node.Metadata["entity_id"].(string)
	if graphContext.Node.Kind != KindResource || resourceID == "" {
		return Context{}, "", ErrInvalid
	}
	return graphContext, resourceID, nil
}
