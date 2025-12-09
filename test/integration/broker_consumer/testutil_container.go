package broker_consumer_integration

// testutil_container.go provides utilities for setting up and managing
// the Google Pub/Sub emulator container for integration tests.

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/openshift-hyperfleet/hyperfleet-adapter/test/integration/testutil"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// PubSubEmulatorImage is the Docker image for the Google Pub/Sub emulator
	PubSubEmulatorImage = "gcr.io/google.com/cloudsdktool/cloud-sdk:emulators"

	// PubSubEmulatorPort is the port the emulator listens on
	PubSubEmulatorPort = "8085/tcp"

	// PubSubEmulatorReadyLog is the log message indicating the emulator is ready
	PubSubEmulatorReadyLog = "[pubsub] This is the Google Pub/Sub fake."
)

// setupPubSubEmulatorContainer starts a Google Pub/Sub emulator container
// and returns the project ID, emulator host, and cleanup function.
// The container is automatically stopped and removed after the test completes
// (including on test failure) using t.Cleanup().
func setupPubSubEmulatorContainer(t *testing.T) (string, string, func()) {
	t.Log("========================================")
	t.Log("Starting Google Pub/Sub emulator container...")
	t.Log("Note: First run will download ~2GB image (this may take several minutes)")
	t.Log("========================================")

	projectID := "test-project"

	// Configure container using shared utility
	config := testutil.DefaultContainerConfig()
	config.Name = "Pub/Sub emulator"
	config.Image = PubSubEmulatorImage
	config.ExposedPorts = []string{PubSubEmulatorPort}
	config.Cmd = []string{
		"gcloud",
		"beta",
		"emulators",
		"pubsub",
		"start",
		fmt.Sprintf("--project=%s", projectID),
		"--host-port=0.0.0.0:8085",
	}
	// Wait for both the log message and the port to be listening
	config.WaitStrategy = testutil.WaitStrategies.ForLogAndPort(
		PubSubEmulatorReadyLog,
		PubSubEmulatorPort,
		180*time.Second,
	)

	t.Log("Pulling/starting container (this may take a while on first run)...")
	result, err := testutil.StartContainer(t, config)
	require.NoError(t, err, "Failed to start Pub/Sub emulator container")

	emulatorHost := result.GetEndpoint(PubSubEmulatorPort)
	t.Logf("Pub/Sub emulator started: %s (project: %s)", emulatorHost, projectID)

	// Return a no-op cleanup function for backwards compatibility with existing tests
	// The actual cleanup is handled by t.Cleanup() in testutil.StartContainer
	cleanup := func() {
		// No-op: cleanup is now handled by testutil.StartContainer
	}

	return projectID, emulatorHost, cleanup
}

// createTopicAndSubscription creates a topic and subscription in the Pub/Sub emulator.
// This must be called before tests can publish/subscribe to topics.
func createTopicAndSubscription(t *testing.T, projectID, topicID, subscriptionID string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create Pub/Sub client (will use PUBSUB_EMULATOR_HOST env var automatically)
	client, err := pubsub.NewClient(ctx, projectID)
	require.NoError(t, err, "Failed to create Pub/Sub client")
	defer func() {
		if err := client.Close(); err != nil {
			t.Logf("Warning: Failed to close Pub/Sub client: %v", err)
		}
	}()

	// Build fully-qualified resource names
	topicName := fmt.Sprintf("projects/%s/topics/%s", projectID, topicID)
	subscriptionName := fmt.Sprintf("projects/%s/subscriptions/%s", projectID, subscriptionID)

	// Create topic using TopicAdminClient
	_, err = client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{
		Name: topicName,
	})
	if err != nil {
		if isAlreadyExistsError(err) {
			t.Logf("Topic already exists: %s", topicID)
		} else {
			require.NoError(t, err, "Failed to create topic %s", topicID)
		}
	} else {
		t.Logf("Created topic: %s", topicID)
	}

	// Create subscription using SubscriptionAdminClient
	_, err = client.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
		Name:               subscriptionName,
		Topic:              topicName,
		AckDeadlineSeconds: 10,
	})
	if err != nil {
		if isAlreadyExistsError(err) {
			t.Logf("Subscription already exists: %s", subscriptionID)
		} else {
			require.NoError(t, err, "Failed to create subscription %s", subscriptionID)
		}
	} else {
		t.Logf("Created subscription: %s", subscriptionID)
	}
}

// isAlreadyExistsError checks if an error is an "AlreadyExists" gRPC error
func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	// Check gRPC status code
	if s, ok := status.FromError(err); ok {
		return s.Code() == codes.AlreadyExists
	}
	// Fallback: check error message
	return strings.Contains(strings.ToLower(err.Error()), "already exists")
}

