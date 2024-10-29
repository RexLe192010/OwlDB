package document

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RICE-COMP318-FALL24/owldb-p1group70/collection"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/patcher"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/stretchr/testify/assert"
)

// Helper function to create a new document for testing
func createTestDocument() Document {
	return New("/test/path", "testUser", map[string]interface{}{"key": "value"})
}

// Test New function
func TestNew(t *testing.T) {
	doc := createTestDocument()
	assert.Equal(t, "/test/path", doc.output.Path)
	assert.Equal(t, "testUser", doc.output.Meta.CreatedBy)
	assert.NotNil(t, doc.children)
}

// Test GetDoc function
func TestGetDoc(t *testing.T) {
	doc := createTestDocument()
	r := httptest.NewRequest(http.MethodGet, "/test/path", nil)
	w := httptest.NewRecorder()

	doc.GetDoc(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
}

// Test PutColl function
func TestPutColl(t *testing.T) {
	doc := createTestDocument()
	r := httptest.NewRequest(http.MethodPut, "/test/path", nil)
	w := httptest.NewRecorder()

	newColl := collection.New()
	doc.PutColl(w, r, "newColl", &newColl)

	// Check if the collection was added to the document
	_, exists := doc.GetColl("newColl")
	assert.True(t, exists)
}

// Test GetColl function
func TestGetColl(t *testing.T) {
	doc := createTestDocument()
	newColl := collection.New()

	// use httptest package to create an http.Request
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/newColl", nil)

	// add the new collection to the document
	doc.PutColl(w, r, "newColl", &newColl)

	coll, exists := doc.GetColl("newColl")
	assert.True(t, exists)
	assert.Equal(t, &newColl, coll)
}

// Test OverwriteBody function
func TestOverwriteBody(t *testing.T) {
	doc := createTestDocument()
	newBody := map[string]interface{}{"newKey": "newValue"}
	doc.OverwriteBody(newBody, "newUser")

	assert.Equal(t, newBody, doc.output.Doc)
	assert.Equal(t, "newUser", doc.output.Meta.LastModifiedBy)
}

// Test ApplyPatches function
func TestApplyPatches(t *testing.T) {
	doc := createTestDocument()

	patchData := []patcher.Patch{
		{Operation: "ObjectAdd", Path: "/newKey", Value: "newValue"},
	}
	schema := &jsonschema.Schema{}

	result, newDoc := doc.ApplyPatches(patchData, schema)

	assert.False(t, result.PatchFailed)
	assert.Equal(t, "Patches applied successfully", result.Message)
	assert.NotNil(t, newDoc)
}

// Test GetLastModified function
func TestGetLastModified(t *testing.T) {
	doc := createTestDocument()
	lastModified := doc.GetLastModified()

	assert.Equal(t, doc.output.Meta.LastModifiedAt, lastModified)
}

// Test GetOriginalAuthor function
func TestGetOriginalAuthor(t *testing.T) {
	doc := createTestDocument()
	originalAuthor := doc.GetOriginalAuthor()

	assert.Equal(t, doc.output.Meta.CreatedBy, originalAuthor)
}

// Test GetJSONBody function
func TestGetJSONBody(t *testing.T) {
	doc := createTestDocument()
	jsonBody, err := doc.GetJSONBody()

	assert.NoError(t, err)

	var output docOutput
	err = json.Unmarshal(jsonBody, &output)
	assert.NoError(t, err)
	assert.Equal(t, doc.output, output)
}

// Test GetRawDoc function
func TestGetRawDoc(t *testing.T) {
	doc := createTestDocument()
	rawDoc := doc.GetRawDoc()

	assert.Equal(t, doc.output, rawDoc)
}

// Test GetJSONDoc function
func TestGetJSONDoc(t *testing.T) {
	doc := createTestDocument()
	jsonDoc := doc.GetJSONDoc()

	assert.Equal(t, doc.output.Doc, jsonDoc)
}
