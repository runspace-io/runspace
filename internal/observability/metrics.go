package observability

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
)

type Metrics struct {
	requests atomic.Uint64
	errors   atomic.Uint64
	events   atomic.Uint64
}

func New() *Metrics          { return &Metrics{} }
func (m *Metrics) IncEvent() { m.events.Add(1) }
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		capture := &statusWriter{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(capture, request)
		m.requests.Add(1)
		if capture.status >= http.StatusInternalServerError {
			m.errors.Add(1)
		}
	})
}
func (m *Metrics) Handler(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "text/plain; version=0.0.4")
	requests, errors, events := m.requests.Load(), m.errors.Load(), m.events.Load()
	_, _ = fmt.Fprintf(writer, "runspace_http_requests_total %d\nrunspace_http_errors_total %d\nrunspace_events_total %d\nforge_http_requests_total %d\nforge_http_errors_total %d\nforge_events_total %d\n", requests, errors, events, requests, errors, events)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (w *statusWriter) ReadFrom(reader io.Reader) (int64, error) {
	if source, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return source.ReadFrom(reader)
	}
	return io.Copy(w.ResponseWriter, reader)
}
