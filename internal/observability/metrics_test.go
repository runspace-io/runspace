package observability

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsMiddlewareAndHandler(t *testing.T) {
	metrics := New()
	handler := metrics.Middleware(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "failed", http.StatusInternalServerError)
	}))
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	metrics.IncEvent()
	metricsResponse := httptest.NewRecorder()
	metrics.Handler(metricsResponse, request)
	body, err := io.ReadAll(metricsResponse.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "forge_http_requests_total 1") || !strings.Contains(string(body), "forge_events_total 1") {
		t.Fatalf("metrics=%s", body)
	}
}
