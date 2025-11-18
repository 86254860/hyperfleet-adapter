//go:build integration
// +build integration

package k8sclient_integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"k8s.io/client-go/rest"

	k8sclient "github.com/openshift-hyperfleet/hyperfleet-adapter/internal/k8s-client"
	"github.com/openshift-hyperfleet/hyperfleet-adapter/pkg/logger"
)

// TestEnvPrebuilt holds the test environment for pre-built image integration tests
type TestEnvPrebuilt struct {
	Container testcontainers.Container
	Client    *k8sclient.Client
	Config    *rest.Config
	Ctx       context.Context
	Log       logger.Logger
}

// Cleanup terminates the container and cleans up resources
func (e *TestEnvPrebuilt) Cleanup(t *testing.T) {
	t.Helper()
	if e.Container != nil {
		if err := e.Container.Terminate(e.Ctx); err != nil {
			t.Logf("Warning: Failed to terminate container: %v", err)
		}
	}
}

// SetupTestEnvPrebuilt sets up integration tests using a pre-built image with envtest.
// This approach avoids Exec() calls and works reliably with both Docker and Podman.
//
// IMPORTANT: INTEGRATION_ENVTEST_IMAGE environment variable must be set.
// Do not call this function directly. Instead, use:
//   make test-integration
//
// The Makefile will automatically:
// - Build the image if needed (using test/Dockerfile.integration)
// - Set INTEGRATION_ENVTEST_IMAGE to the appropriate value
// - Run the integration tests
//
// For CI/CD, set INTEGRATION_ENVTEST_IMAGE to your pre-built image:
//   INTEGRATION_ENVTEST_IMAGE=quay.io/your-org/integration-test:v1 make test-integration
func SetupTestEnvPrebuilt(t *testing.T) *TestEnvPrebuilt {
	t.Helper()

	ctx := context.Background()
	log := logger.NewLogger(ctx)

	// Check that INTEGRATION_ENVTEST_IMAGE is set
	imageName := os.Getenv("INTEGRATION_ENVTEST_IMAGE")
	if imageName == "" {
		t.Fatalf(`INTEGRATION_ENVTEST_IMAGE environment variable is not set.

Please run integration tests using:
  make test-integration

The Makefile will automatically build the image if needed and set INTEGRATION_ENVTEST_IMAGE.

For CI/CD environments, set INTEGRATION_ENVTEST_IMAGE to your pre-built image:
  INTEGRATION_ENVTEST_IMAGE=quay.io/your-org/integration-test:v1 make test-integration`)
	}
	log.Infof("Using integration image: %s", imageName)

	// Create container with timeout
	containerCtx, containerCancel := context.WithTimeout(ctx, 3*time.Minute)
	defer containerCancel()

	// Configure proxy settings from environment
	httpProxy := os.Getenv("HTTP_PROXY")
	httpsProxy := os.Getenv("HTTPS_PROXY")
	noProxy := os.Getenv("NO_PROXY")

	if httpProxy != "" {
		log.Infof("Configuring HTTP_PROXY: %s", httpProxy)
	}
	if httpsProxy != "" {
		log.Infof("Configuring HTTPS_PROXY: %s", httpsProxy)
	}

	// Container request with pre-built image
	req := testcontainers.ContainerRequest{
		Image:        imageName,
		ExposedPorts: []string{"6443/tcp"},
		Env: map[string]string{
			"HTTP_PROXY":  httpProxy,
			"HTTPS_PROXY": httpsProxy,
			"NO_PROXY":    noProxy,
		},
		// Reserve memory for kube-apiserver (needs ~300MB minimum)
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Memory = 512 * 1024 * 1024        // 512MB memory limit
			hc.MemoryReservation = 256 * 1024 * 1024  // 256MB soft limit
		},
		// Use TCP wait + log-based wait since health endpoints require auth
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("6443/tcp"),
			wait.ForLog("Envtest is running"),
		).WithStartupTimeout(90 * time.Second),
	}

	log.Infof("Creating container from image: %s", imageName)
	log.Infof("This should be fast since binaries are pre-installed...")

	container, err := testcontainers.GenericContainer(containerCtx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		if containerCtx.Err() == context.DeadlineExceeded {
			t.Fatalf(`Container creation timed out after 3 minutes.

Image: %s

Possible causes:
1. Image pull is stuck (check network/proxy configuration)
2. Container runtime is slow or unresponsive
3. Image does not exist (ensure 'make test-integration' was used)`, imageName)
		}
		t.Fatalf("Failed to start container: %v", err)
	}
	require.NotNil(t, container, "Container is nil")

	log.Infof("Container started successfully")

	// Get the mapped port for kube-apiserver
	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "6443/tcp")
	require.NoError(t, err)

	kubeAPIServer := fmt.Sprintf("https://%s:%s", host, port.Port())
	log.Infof("Kube-apiserver available at: %s", kubeAPIServer)

	// Give API server a moment to fully initialize
	log.Infof("Waiting for API server to be fully ready...")
	time.Sleep(5 * time.Second)
	log.Infof("API server is ready!")

	// Create Kubernetes client using the k8s-client package
	log.Infof("Creating Kubernetes client...")

	// Create rest.Config for the client with bearer token authentication
	config := &rest.Config{
		Host:        kubeAPIServer,
		BearerToken: "test-token", // Matches token in /tmp/envtest/certs/token-auth-file
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true, // Skip TLS verification for testing
		},
	}

	// Create client using the config
	client, err := k8sclient.NewClientFromConfig(ctx, config, log)
	require.NoError(t, err)
	require.NotNil(t, client)

	log.Infof("Kubernetes client created successfully")

	return &TestEnvPrebuilt{
		Container: container,
		Client:    client,
		Config:    config,
		Ctx:       ctx,
		Log:       log,
	}
}

