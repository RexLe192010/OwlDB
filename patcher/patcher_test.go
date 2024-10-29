package patcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// test object add operation of patch
func TestApplyPatch_ObjectAdd(t *testing.T) {
	doc := map[string]interface{}{
		"name": "Rex",
		"age":  20,
	}

	patch := Patch{
		Operation: "ObjectAdd",
		Path:      "/address",
		Value:     "9 Sunset Blvd",
	}

	expected := map[string]interface{}{
		"name":    "Rex",
		"age":     20,
		"address": "9 Sunset Blvd",
	}

	patchedDoc, err := ApplyPatch(doc, patch)
	assert.NoError(t, err)
	assert.Equal(t, expected, patchedDoc)
}

// test array add operation of patch
func TestApplyPatch_ArrayAdd(t *testing.T) {
	doc := map[string]interface{}{
		"items": []interface{}{"item1", "item2"},
	}

	patch := Patch{
		Operation: "ArrayAdd",
		Path:      "/items",
		Value:     "item3",
	}

	expected := map[string]interface{}{
		"items": []interface{}{"item1", "item2", "item3"},
	}

	patchedDoc, err := ApplyPatch(doc, patch)
	assert.NoError(t, err)
	assert.Equal(t, expected, patchedDoc)
}

// test array remove operation of patch
func TestApplyPatch_ArrayRemove(t *testing.T) {
	doc := map[string]interface{}{
		"items": []interface{}{"item1", "item2", "item3"},
	}

	patch := Patch{
		Operation: "ArrayRemove",
		Path:      "/items",
		Value:     "item2",
	}

	expected := map[string]interface{}{
		"items": []interface{}{"item1", "item3"},
	}

	patchedDoc, err := ApplyPatch(doc, patch)
	assert.NoError(t, err)
	assert.Equal(t, expected, patchedDoc)
}

// test invalid patch operation
func TestApplyPatch_InvalidOperation(t *testing.T) {
	doc := map[string]interface{}{
		"name": "Xer",
	}

	patch := Patch{
		Operation: "InvalidOp",
		Path:      "/name",
		Value:     "Rex",
	}

	_, err := ApplyPatch(doc, patch)
	assert.Error(t, err)
}

// test key not found in patch
func TestApplyPatch_KeyNotFound(t *testing.T) {
	doc := map[string]interface{}{
		"name": "Xer",
	}

	patch := Patch{
		Operation: "ObjectAdd",
		Path:      "/nonexistent/key",
		Value:     "value",
	}

	_, err := ApplyPatch(doc, patch)
	assert.Error(t, err)
}

// test index out of bounds in patch
func TestApplyPatch_IndexOutOfBounds(t *testing.T) {
	doc := map[string]interface{}{
		"items": []interface{}{"item1", "item2"},
	}

	patch := Patch{
		Operation: "ArrayAdd",
		Path:      "/items/10",
		Value:     "item3",
	}

	_, err := ApplyPatch(doc, patch)
	assert.Error(t, err)
}

// Test ApplyPatch errors on an invalid path
func TestApplyPatch_BadPath(t *testing.T) {
	// Document setup
	patchedDoc := map[string]interface{}{
		"a": 1,
		"b": map[string]interface{}{
			"c": true,
		},
		"array": []interface{}{1, 2, 3},
	}

	patch := Patch{
		Operation: "ObjectAdd",
		Path:      "/c/d", // Invalid path, 'c' doesn't exist
		Value:     100,
	}

	_, err := ApplyPatch(patchedDoc, patch)
	assert.Error(t, err) // Expecting an error due to invalid path
}

// Test ApplyPatch handles missing leading slash
func TestApplyPatch_MissingSlash(t *testing.T) {
	// Document setup
	patchedDoc := map[string]interface{}{
		"a": 1,
		"b": map[string]interface{}{
			"c": true,
		},
		"array": []interface{}{1, 2, 3},
	}

	patch := Patch{
		Operation: "ObjectAdd",
		Path:      "c", // Path missing leading slash
		Value:     100,
	}

	_, err := ApplyPatch(patchedDoc, patch)
	assert.Error(t, err) // Expecting an error due to missing leading slash
}

// Test ApplyPatch does not modify the document on error
func TestApplyPatch_NotModifiedOnError(t *testing.T) {
	// Original document setup
	doc := map[string]interface{}{
		"a": 1,
		"b": map[string]interface{}{
			"c": true,
		},
		"array": []interface{}{1, 2, 3},
	}

	// Clone of original document for patching
	patchedDoc := map[string]interface{}{
		"a": 1,
		"b": map[string]interface{}{
			"c": true,
		},
		"array": []interface{}{1, 2, 3},
	}

	patch := Patch{
		Operation: "ObjectAdd",
		Path:      "/invalid/path", // Invalid path should cause error
		Value:     100,
	}

	_, err := ApplyPatch(patchedDoc, patch)
	assert.Error(t, err)             // Expecting an error
	assert.Equal(t, doc, patchedDoc) // Ensure doc remains unchanged
}

// Test ApplyPatch does not modify the original document after a successful patch
func TestApplyPatch_NotModifiedOnSuccess(t *testing.T) {
	// Original document setup
	doc := map[string]interface{}{
		"a": 1,
		"b": map[string]interface{}{
			"c": true,
		},
		"array": []interface{}{1, 2, 3},
	}

	// Clone of original document for patching
	patchedDoc := map[string]interface{}{
		"a": 1,
		"b": map[string]interface{}{
			"c": true,
		},
		"array": []interface{}{1, 2, 3},
	}

	patch := Patch{
		Operation: "ObjectAdd",
		Path:      "/c", // Valid patch
		Value:     100,
	}

	_, err := ApplyPatch(patchedDoc, patch)
	assert.NoError(t, err)           // Expect no error
	assert.Equal(t, doc, patchedDoc) // Ensure original doc remains unchanged

}
