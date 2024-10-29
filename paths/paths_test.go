package paths

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/RICE-COMP318-FALL24/owldb-p1group70/collection"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/collectionholder"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/document"
	"github.com/stretchr/testify/assert"
)

// Tests for ParsePath

func TestParsePath_ValidPath(t *testing.T) {
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	// Setting up a valid collection holder
	doc := document.New("/v1/db/doc1", "testUser", map[string]interface{}{"key": "value"})
	coll := collection.New()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/v1/db/doc1", nil)
	coll.PutDoc(w, r, "/v1/db/doc1", &doc)
	holder := collectionholder.New()
	holder.PutColl(w, r, "/v1/db", &coll)

	// Prepare HTTP request and recorder
	r = httptest.NewRequest("GET", "/v1/db/doc1", nil)

	// Test valid path /v1/db/doc1
	collResult, docResult, resCode := ParsePath(r.URL.Path, &holder)
	slog.Debug("TestParsePath_ValidPath", "collResult", collResult, "docResult", docResult, "resCode", resCode)

	// Assert correct parsing and document retrieval
	assert.Nil(t, collResult)
	assert.NotNil(t, docResult)
	assert.Equal(t, docResult.GetRawDoc(), map[string]interface{}{"key": "value"})
	assert.Equal(t, RESOURCE_DOC, resCode)
}

func TestParsePath_InvalidVersion(t *testing.T) {
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	holder := collectionholder.New()

	// Prepare HTTP request and recorder
	// w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/v2/db", nil)

	// Test invalid path /v2/db
	coll, doc, resCode := ParsePath(r.URL.Path, &holder)

	// Assert correct handling of invalid version
	assert.Nil(t, coll)
	assert.Nil(t, doc)
	assert.Equal(t, ERROR_NO_VERSION, resCode)
}

func TestParsePath_CollectionPath(t *testing.T) {
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	// Setting up collection holder with collections
	coll := collection.New()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/v1/db/", nil)
	holder := collectionholder.New()
	holder.PutColl(w, r, "/v1/db", &coll)

	// Prepare HTTP request and recorder
	r = httptest.NewRequest("GET", "/v1/db/", nil)

	// Test valid collection path /v1/db/
	collResult, docResult, resCode := ParsePath(r.URL.Path, &holder)

	// Assert correct collection retrieval
	assert.NotNil(t, collResult)
	assert.Nil(t, docResult)
	assert.Equal(t, RESOURCE_DB, resCode)
}

func TestParsePath_DocumentNotFound(t *testing.T) {
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	coll := collection.New()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/v1/db/", nil)
	holder := collectionholder.New()
	holder.PutColl(w, r, "/v1/db", &coll)

	// Prepare HTTP request and recorder
	r = httptest.NewRequest("GET", "/v1/db/doc2", nil)

	// Test path to a non-existent document /v1/db/doc2
	collResult, docResult, resCode := ParsePath(r.URL.Path, &holder)

	// Assert document not found
	assert.Nil(t, collResult)
	assert.Nil(t, docResult)
	assert.Equal(t, ERROR_NO_DOC, resCode)
}

// Tests for GetParentResource

func TestGetParentResource_ValidDoc(t *testing.T) {
	// Prepare HTTP request and recorder
	// w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/v1/db/coll/doc", nil)

	// Test extracting parent of /v1/db/coll/doc
	truncated, parent, resCode := GetParentResource(r.URL.Path)

	// Assert correct extraction
	assert.Equal(t, "/v1/db/coll/", truncated)
	assert.Equal(t, "doc", parent)
	assert.Equal(t, RESOURCE_DOC, resCode)
}

func TestGetParentResource_Collection(t *testing.T) {
	// Prepare HTTP request and recorder
	// w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/v1/db/coll/", nil)

	// Test extracting parent of /v1/db/coll/
	truncated, parent, resCode := GetParentResource(r.URL.Path)

	// Assert correct extraction
	assert.Equal(t, "/v1/db/", truncated)
	assert.Equal(t, "coll", parent)
	assert.Equal(t, RESOURCE_COLL, resCode)
}

// Tests for HandlePathError

func TestHandlePathError_InvalidSlash(t *testing.T) {
	// Prepare HTTP request and recorder for testing
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/v1/db//coll/", nil)

	// Call the error handler
	HandlePathError(w, r, ERROR_BAD_SLASH)

	// Assert that the correct status and message are returned
	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	assert.Contains(t, w.Body.String(), "Invalid path: Bad slash")
}

func TestHandlePathError_NoVersion(t *testing.T) {
	// Prepare HTTP request and recorder
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/v2/db", nil)

	// Call the error handler
	HandlePathError(w, r, ERROR_NO_VERSION)

	// Assert correct status and response
	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	assert.Contains(t, w.Body.String(), "Invalid path: No version")
}

// Tests for Path Manipulation Functions

func TestGetRelativePathNonDB(t *testing.T) {
	// Prepare HTTP request and recorder
	// w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/v1/db/coll/doc", nil)

	// Test /v1/db/coll/doc should return /coll/doc
	relativePath := GetRelativePathNonDB(r.URL.Path)
	assert.Equal(t, "/coll/doc", relativePath)
}

func TestGetRelativePathDB(t *testing.T) {
	// Prepare HTTP request and recorder
	// w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/v1/db/coll/doc", nil)

	// Test /v1/db/coll/doc should return /db/coll/doc
	relativePath := GetRelativePathDB(r.URL.Path)
	assert.Equal(t, "/db/coll/doc", relativePath)
}
