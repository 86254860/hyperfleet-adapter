//go:build integration
// +build integration

package k8sclient_integration

import (
	"context"
	"os"
	"testing"

	k8sclient "github.com/openshift-hyperfleet/hyperfleet-adapter/internal/k8s-client"
	"github.com/openshift-hyperfleet/hyperfleet-adapter/pkg/logger"
	"k8s.io/client-go/rest"
)

// TestEnv is a common interface for all integration test environments
type TestEnv interface {
	GetClient() *k8sclient.Client
	GetConfig() *rest.Config
	GetContext() context.Context
	GetLogger() logger.Logger
	Cleanup(t *testing.T)
}

// Ensure both implementations satisfy the interface
var _ TestEnv = (*TestEnvPrebuilt)(nil)
var _ TestEnv = (*TestEnvK3s)(nil)

// TestEnvK3s wraps TestEnvTestcontainers to implement TestEnv interface
type TestEnvK3s struct {
	*TestEnvTestcontainers
}

func (e *TestEnvK3s) GetClient() *k8sclient.Client {
	return e.Client
}

func (e *TestEnvK3s) GetConfig() *rest.Config {
	return nil // K3s doesn't expose Config in current implementation
}

func (e *TestEnvK3s) GetContext() context.Context {
	return e.Ctx
}

func (e *TestEnvK3s) GetLogger() logger.Logger {
	return e.Log
}

// GetClient returns the k8s client
func (e *TestEnvPrebuilt) GetClient() *k8sclient.Client {
	return e.Client
}

// GetConfig returns the rest config
func (e *TestEnvPrebuilt) GetConfig() *rest.Config {
	return e.Config
}

// GetContext returns the context
func (e *TestEnvPrebuilt) GetContext() context.Context {
	return e.Ctx
}

// GetLogger returns the logger
func (e *TestEnvPrebuilt) GetLogger() logger.Logger {
	return e.Log
}

// SetupTestEnv creates a test environment based on INTEGRATION_STRATEGY env var
// - If INTEGRATION_STRATEGY=k3s, uses K3s testcontainers (privileged, slower, more realistic)
// - Otherwise, uses pre-built envtest image (unprivileged, faster, suitable for CI/CD)
func SetupTestEnv(t *testing.T) TestEnv {
	t.Helper()

	strategy := os.Getenv("INTEGRATION_STRATEGY")
	
	switch strategy {
	case "k3s":
		t.Logf("Using K3s integration test strategy")
		tcEnv := SetupTestEnvTestcontainers(t)
		return &TestEnvK3s{TestEnvTestcontainers: tcEnv}
	default:
		t.Logf("Using pre-built envtest integration test strategy")
		return SetupTestEnvPrebuilt(t)
	}
}

