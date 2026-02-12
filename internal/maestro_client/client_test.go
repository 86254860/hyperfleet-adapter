package maestro_client

import (
	"encoding/json"
	"testing"

	"github.com/openshift-hyperfleet/hyperfleet-adapter/internal/transport_client"
	"github.com/openshift-hyperfleet/hyperfleet-adapter/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	workv1 "open-cluster-management.io/api/work/v1"
)

// --- helpers ---

// mustJSON marshals v to JSON or panics.
func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}

// bareNamespaceJSON returns a bare Namespace manifest as JSON.
func bareNamespaceJSON(t *testing.T, name string) []byte {
	t.Helper()
	return mustJSON(t, map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]interface{}{
			"name": name,
			"annotations": map[string]interface{}{
				constants.AnnotationGeneration: "1",
			},
		},
	})
}

// unmarshalManifestRaw unmarshals a workv1.Manifest.Raw back to a map.
func unmarshalManifestRaw(t *testing.T, m workv1.Manifest) map[string]interface{} {
	t.Helper()
	require.NotNil(t, m.Raw)
	var obj map[string]interface{}
	require.NoError(t, json.Unmarshal(m.Raw, &obj))
	return obj
}

// newTestTemplate creates a ManifestWork template with the given workload manifests.
func newTestTemplate(name string, manifests []workv1.Manifest) *workv1.ManifestWork {
	return &workv1.ManifestWork{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				constants.AnnotationGeneration: "1",
			},
			Labels: map[string]string{
				"test": "true",
			},
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: manifests,
			},
		},
	}
}

// --- buildManifestWork tests ---

func TestBuildManifestWork_ExplicitResources(t *testing.T) {
	// When resources have non-nil Manifest, template workload manifests are replaced.
	templateManifests := []workv1.Manifest{
		{RawExtension: runtime.RawExtension{Raw: bareNamespaceJSON(t, "template-ns")}},
	}
	template := newTestTemplate("test-mw", templateManifests)

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "explicit-cm",
				"namespace": "default",
			},
		},
	}

	c := &Client{}
	work, err := c.buildManifestWork(template, []transport_client.ResourceToApply{
		{Name: "cm", Manifest: resource},
	}, "consumer-1")

	require.NoError(t, err)
	assert.Equal(t, "consumer-1", work.Namespace)
	assert.Equal(t, "test-mw", work.Name)
	require.Len(t, work.Spec.Workload.Manifests, 1)

	obj := unmarshalManifestRaw(t, work.Spec.Workload.Manifests[0])
	assert.Equal(t, "ConfigMap", obj["kind"], "should contain the explicit resource, not the template's")
}

func TestBuildManifestWork_NilManifestUsesTemplate(t *testing.T) {
	// When all resources have nil Manifest, template workload manifests are used as-is.
	templateManifests := []workv1.Manifest{
		{RawExtension: runtime.RawExtension{Raw: bareNamespaceJSON(t, "from-template")}},
	}
	template := newTestTemplate("test-mw", templateManifests)

	c := &Client{}
	work, err := c.buildManifestWork(template, []transport_client.ResourceToApply{
		{Name: "ns", Manifest: nil},
	}, "consumer-1")

	require.NoError(t, err)
	require.Len(t, work.Spec.Workload.Manifests, 1)

	obj := unmarshalManifestRaw(t, work.Spec.Workload.Manifests[0])
	assert.Equal(t, "Namespace", obj["kind"])
	assert.Equal(t, "v1", obj["apiVersion"])
}

func TestBuildManifestWork_EmptyResources(t *testing.T) {
	// Empty resources list should use template manifests.
	templateManifests := []workv1.Manifest{
		{RawExtension: runtime.RawExtension{Raw: bareNamespaceJSON(t, "keep-me")}},
	}
	template := newTestTemplate("test-mw", templateManifests)

	c := &Client{}
	work, err := c.buildManifestWork(template, []transport_client.ResourceToApply{}, "consumer-1")

	require.NoError(t, err)
	require.Len(t, work.Spec.Workload.Manifests, 1)

	obj := unmarshalManifestRaw(t, work.Spec.Workload.Manifests[0])
	assert.Equal(t, "Namespace", obj["kind"])
}

func TestBuildManifestWork_DoesNotMutateTemplate(t *testing.T) {
	// The original template must not be modified.
	originalJSON := bareNamespaceJSON(t, "original-ns")
	templateManifests := []workv1.Manifest{
		{RawExtension: runtime.RawExtension{Raw: originalJSON}},
	}
	template := newTestTemplate("test-mw", templateManifests)

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]interface{}{"name": "new-cm", "namespace": "default"},
		},
	}

	c := &Client{}
	_, err := c.buildManifestWork(template, []transport_client.ResourceToApply{
		{Name: "cm", Manifest: resource},
	}, "consumer-1")
	require.NoError(t, err)

	// Template should still have the original Namespace manifest
	require.Len(t, template.Spec.Workload.Manifests, 1)
	obj := unmarshalManifestRaw(t, template.Spec.Workload.Manifests[0])
	assert.Equal(t, "Namespace", obj["kind"])
	assert.Equal(t, "", template.Namespace, "template namespace should not be modified")
}

func TestBuildManifestWork_SetsConsumerNamespace(t *testing.T) {
	template := newTestTemplate("test-mw", []workv1.Manifest{
		{RawExtension: runtime.RawExtension{Raw: bareNamespaceJSON(t, "ns")}},
	})

	c := &Client{}
	work, err := c.buildManifestWork(template, []transport_client.ResourceToApply{}, "my-cluster")

	require.NoError(t, err)
	assert.Equal(t, "my-cluster", work.Namespace)
}

func TestBuildManifestWork_PreservesMetadata(t *testing.T) {
	template := newTestTemplate("my-manifestwork", []workv1.Manifest{
		{RawExtension: runtime.RawExtension{Raw: bareNamespaceJSON(t, "ns")}},
	})
	template.Labels["extra"] = "label"
	template.Annotations["extra"] = "annotation"

	c := &Client{}
	work, err := c.buildManifestWork(template, []transport_client.ResourceToApply{}, "consumer-1")

	require.NoError(t, err)
	assert.Equal(t, "my-manifestwork", work.Name)
	assert.Equal(t, "true", work.Labels["test"])
	assert.Equal(t, "label", work.Labels["extra"])
	assert.Equal(t, "1", work.Annotations[constants.AnnotationGeneration])
	assert.Equal(t, "annotation", work.Annotations["extra"])
}

func TestBuildManifestWork_MixedNilAndExplicitResources(t *testing.T) {
	// If at least one resource has a non-nil Manifest, template manifests are replaced.
	template := newTestTemplate("test-mw", []workv1.Manifest{
		{RawExtension: runtime.RawExtension{Raw: bareNamespaceJSON(t, "template-ns")}},
	})

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]interface{}{"name": "explicit-cm", "namespace": "default"},
		},
	}

	c := &Client{}
	work, err := c.buildManifestWork(template, []transport_client.ResourceToApply{
		{Name: "skipped", Manifest: nil},
		{Name: "cm", Manifest: resource},
	}, "consumer-1")

	require.NoError(t, err)
	// Only the explicit resource should be included (nil ones are skipped)
	require.Len(t, work.Spec.Workload.Manifests, 1)

	obj := unmarshalManifestRaw(t, work.Spec.Workload.Manifests[0])
	assert.Equal(t, "ConfigMap", obj["kind"])
}

func TestBuildManifestWork_TemplateWithMultipleBareManifests(t *testing.T) {
	// Simulates the real-world scenario: ManifestWork template with Namespace + ConfigMap
	// as bare manifests, and nil Manifest resources.
	nsJSON := bareNamespaceJSON(t, "cluster-abc")
	cmJSON := mustJSON(t, map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "cluster-config",
			"namespace": "cluster-abc",
			"annotations": map[string]interface{}{
				constants.AnnotationGeneration: "1",
			},
		},
		"data": map[string]interface{}{"cluster_id": "abc"},
	})

	template := newTestTemplate("hyperfleet-cluster-setup-abc", []workv1.Manifest{
		{RawExtension: runtime.RawExtension{Raw: nsJSON}},
		{RawExtension: runtime.RawExtension{Raw: cmJSON}},
	})

	c := &Client{}
	work, err := c.buildManifestWork(template, []transport_client.ResourceToApply{
		{Name: "manifestwork", Manifest: nil},
	}, "cluster1")

	require.NoError(t, err)
	assert.Equal(t, "cluster1", work.Namespace)
	require.Len(t, work.Spec.Workload.Manifests, 2)

	ns := unmarshalManifestRaw(t, work.Spec.Workload.Manifests[0])
	assert.Equal(t, "Namespace", ns["kind"])
	assert.Equal(t, "v1", ns["apiVersion"])
	nsMeta := ns["metadata"].(map[string]interface{})
	assert.Equal(t, "cluster-abc", nsMeta["name"])

	cm := unmarshalManifestRaw(t, work.Spec.Workload.Manifests[1])
	assert.Equal(t, "ConfigMap", cm["kind"])
	cmMeta := cm["metadata"].(map[string]interface{})
	assert.Equal(t, "cluster-config", cmMeta["name"])
	assert.Equal(t, "cluster-abc", cmMeta["namespace"])
}
