package executor

import (
	"context"
	"testing"

	"github.com/openshift-hyperfleet/hyperfleet-adapter/pkg/logger"
	"github.com/stretchr/testify/assert"
)


func TestDeepCopyMap_BasicTypes(t *testing.T) {
	log := &mockLogger{}

	original := map[string]interface{}{
		"string": "hello",
		"int":    42,
		"float":  3.14,
		"bool":   true,
		"null":   nil,
	}

	copied := deepCopyMap(original, log)

	// Verify values are copied correctly
	assert.Equal(t, "hello", copied["string"])
	assert.Equal(t, 42, copied["int"]) // copystructure preserves int (unlike JSON which converts to float64)
	assert.Equal(t, 3.14, copied["float"])
	assert.Equal(t, true, copied["bool"])
	assert.Nil(t, copied["null"])

	// Verify no warnings logged
	assert.Empty(t, log.warnings, "No warnings expected for basic types")

	// Verify mutation doesn't affect original
	copied["string"] = "modified"
	assert.Equal(t, "hello", original["string"], "Original should not be modified")
}

func TestDeepCopyMap_NestedMaps(t *testing.T) {
	log := &mockLogger{}

	original := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"value": "deep",
			},
		},
	}

	copied := deepCopyMap(original, log)

	// Verify deep copy works
	assert.Empty(t, log.warnings)

	// Modify the copied nested map
	level1 := copied["level1"].(map[string]interface{})
	level2 := level1["level2"].(map[string]interface{})
	level2["value"] = "modified"

	// Verify original is NOT modified (deep copy worked)
	originalLevel1 := original["level1"].(map[string]interface{})
	originalLevel2 := originalLevel1["level2"].(map[string]interface{})
	assert.Equal(t, "deep", originalLevel2["value"], "Original nested value should not be modified")
}

func TestDeepCopyMap_Slices(t *testing.T) {
	log := &mockLogger{}

	original := map[string]interface{}{
		"items": []interface{}{"a", "b", "c"},
		"nested": []interface{}{
			map[string]interface{}{"key": "value"},
		},
	}

	copied := deepCopyMap(original, log)

	assert.Empty(t, log.warnings)

	// Modify copied slice
	copiedItems := copied["items"].([]interface{})
	copiedItems[0] = "modified"

	// Verify original is NOT modified
	originalItems := original["items"].([]interface{})
	assert.Equal(t, "a", originalItems[0], "Original slice should not be modified")
}

func TestDeepCopyMap_Channel(t *testing.T) {
	// copystructure handles channels properly (creates new channel)
	log := &mockLogger{}

	ch := make(chan int, 5)
	original := map[string]interface{}{
		"channel": ch,
		"normal":  "value",
	}

	copied := deepCopyMap(original, log)

	// copystructure handles channels - no warning expected
	assert.Empty(t, log.warnings, "copystructure handles channels without falling back to shallow copy")

	// Normal values are copied
	assert.Equal(t, "value", copied["normal"])

	// Verify channel exists in copied map
	copiedCh, ok := copied["channel"].(chan int)
	assert.True(t, ok, "Channel should be present in copied map")
	assert.NotNil(t, copiedCh, "Copied channel should not be nil")
}

func TestDeepCopyMap_Function(t *testing.T) {
	// copystructure handles functions (copies the function pointer)
	log := &mockLogger{}

	fn := func() string { return "hello" }
	original := map[string]interface{}{
		"func":   fn,
		"normal": "value",
	}

	copied := deepCopyMap(original, log)

	// copystructure handles functions - no warning expected
	assert.Empty(t, log.warnings, "copystructure handles functions without falling back to shallow copy")

	// Normal values are copied
	assert.Equal(t, "value", copied["normal"])

	// Function is preserved
	copiedFn := copied["func"].(func() string)
	assert.Equal(t, "hello", copiedFn(), "Copied function should work")
}

func TestDeepCopyMap_NestedWithChannel(t *testing.T) {
	// Test that nested maps are deep copied even when channels are present
	log := &mockLogger{}

	ch := make(chan int)
	nested := map[string]interface{}{"mutable": "original"}
	original := map[string]interface{}{
		"channel": ch,
		"nested":  nested,
	}

	copied := deepCopyMap(original, log)

	// copystructure handles this properly - no warning expected
	assert.Empty(t, log.warnings)

	// Modify the copied nested map
	copiedNested := copied["nested"].(map[string]interface{})
	copiedNested["mutable"] = "MUTATED"

	// Original should NOT be affected (deep copy works with copystructure)
	assert.Equal(t, "original", nested["mutable"],
		"Deep copy: original nested map should NOT be affected by mutation")
}

func TestDeepCopyMap_EmptyMap(t *testing.T) {
	log := &mockLogger{}

	original := map[string]interface{}{}
	copied := deepCopyMap(original, log)

	assert.Empty(t, log.warnings)
	assert.NotNil(t, copied)
	assert.Empty(t, copied)
}

func TestDeepCopyMap_NilLogger(t *testing.T) {
	// Should not panic when logger is nil
	original := map[string]interface{}{
		"string": "value",
		"nested": map[string]interface{}{
			"key": "nested_value",
		},
	}

	// Should not panic even with nil logger
	copied := deepCopyMap(original, nil)

	assert.Equal(t, "value", copied["string"])
	
	// Verify deep copy works
	copiedNested := copied["nested"].(map[string]interface{})
	copiedNested["key"] = "modified"
	
	originalNested := original["nested"].(map[string]interface{})
	assert.Equal(t, "nested_value", originalNested["key"], "Original should not be modified")
}

func TestDeepCopyMap_NilMap(t *testing.T) {
	log := &mockLogger{}

	copied := deepCopyMap(nil, log)

	assert.Nil(t, copied)
	assert.Empty(t, log.warnings)
}

func TestDeepCopyMap_KubernetesManifest(t *testing.T) {
	// Test with a realistic Kubernetes manifest structure
	log := &mockLogger{}

	original := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "test-config",
			"namespace": "default",
			"labels": map[string]interface{}{
				"app": "test",
			},
		},
		"data": map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		},
	}

	copied := deepCopyMap(original, log)

	assert.Empty(t, log.warnings)

	// Modify copied manifest
	copiedMetadata := copied["metadata"].(map[string]interface{})
	copiedLabels := copiedMetadata["labels"].(map[string]interface{})
	copiedLabels["app"] = "modified"

	// Verify original is NOT modified
	originalMetadata := original["metadata"].(map[string]interface{})
	originalLabels := originalMetadata["labels"].(map[string]interface{})
	assert.Equal(t, "test", originalLabels["app"], "Original manifest should not be modified")
}

// TestDeepCopyMap_Context ensures the function is used correctly in context
func TestDeepCopyMap_RealWorldContext(t *testing.T) {
	// This simulates how deepCopyMap is used in executeResource
	log := logger.NewLogger(context.Background())

	manifest := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]interface{}{
			"name": "{{ .namespace }}",
		},
	}

	// Deep copy before template rendering
	copied := deepCopyMap(manifest, log)

	// Simulate template rendering modifying the copy
	copiedMetadata := copied["metadata"].(map[string]interface{})
	copiedMetadata["name"] = "rendered-namespace"

	// Original template should remain unchanged for next iteration
	originalMetadata := manifest["metadata"].(map[string]interface{})
	assert.Equal(t, "{{ .namespace }}", originalMetadata["name"])
}

