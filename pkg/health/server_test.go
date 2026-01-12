package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openshift-hyperfleet/hyperfleet-adapter/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger implements logger.Logger for testing
type mockLogger struct{}

func (m *mockLogger) Debug(ctx context.Context, msg string)                          {}
func (m *mockLogger) Debugf(ctx context.Context, format string, args ...interface{}) {}
func (m *mockLogger) Info(ctx context.Context, msg string)                           {}
func (m *mockLogger) Infof(ctx context.Context, format string, args ...interface{})  {}
func (m *mockLogger) Warn(ctx context.Context, msg string)                           {}
func (m *mockLogger) Warnf(ctx context.Context, format string, args ...interface{})  {}
func (m *mockLogger) Error(ctx context.Context, msg string)                          {}
func (m *mockLogger) Errorf(ctx context.Context, format string, args ...interface{}) {}
func (m *mockLogger) Fatal(ctx context.Context, msg string)                          {}
func (m *mockLogger) With(key string, value interface{}) logger.Logger               { return m }
func (m *mockLogger) WithFields(fields map[string]interface{}) logger.Logger         { return m }
func (m *mockLogger) Without(key string) logger.Logger                               { return m }

func TestHealthzHandler(t *testing.T) {
	server := NewServer(&mockLogger{}, "8080", "test-adapter")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	server.healthzHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var response Response
	err := json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response.Status)
	assert.Empty(t, response.Message)
}

func TestReadyzHandler_NotReady(t *testing.T) {
	server := NewServer(&mockLogger{}, "8080", "test-adapter")
	// By default, ready is false

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	server.readyzHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var response Response
	err := json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "error", response.Status)
	assert.Equal(t, "not ready", response.Message)
}

func TestReadyzHandler_Ready(t *testing.T) {
	server := NewServer(&mockLogger{}, "8080", "test-adapter")
	server.SetReady(true)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	server.readyzHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var response Response
	err := json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response.Status)
	assert.Empty(t, response.Message)
}

func TestSetReady(t *testing.T) {
	server := NewServer(&mockLogger{}, "8080", "test-adapter")

	// Initially not ready
	assert.False(t, server.IsReady())

	// Set ready
	server.SetReady(true)
	assert.True(t, server.IsReady())

	// Set not ready again
	server.SetReady(false)
	assert.False(t, server.IsReady())
}

func TestReadyzHandler_ReadyToNotReady(t *testing.T) {
	server := NewServer(&mockLogger{}, "8080", "test-adapter")

	// Set ready first
	server.SetReady(true)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	server.readyzHandler(w, req)
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)

	// Set not ready (simulating shutdown)
	server.SetReady(false)

	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w = httptest.NewRecorder()
	server.readyzHandler(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Result().StatusCode)
}
