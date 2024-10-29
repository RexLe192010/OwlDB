package collection

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RICE-COMP318-FALL24/owldb-p1group70/document"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/patcher"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/stretchr/testify/assert"
)

// TestGetDoc tests the GetDoc function
func TestGetDoc(t *testing.T) {
	c := New()
	req := httptest.NewRequest(http.MethodGet, "/documents?interval=[a,z]", nil)
	w := httptest.NewRecorder()

	c.GetDoc(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.JSONEq(t, "[]", string(body))
}

// TestPutDoc tests the PutDoc function
func TestPutDoc(t *testing.T) {
	c := New()
	doc := document.New("/documents/1", "user", map[string]interface{}{"key": "value"})
	req := httptest.NewRequest(http.MethodPut, "/documents/1", nil)
	w := httptest.NewRecorder()

	c.PutDoc(w, req, "1", &doc)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.JSONEq(t, `{"uri":"/documents/1"}`, string(body))
}

// TestDeleteDoc tests the DeleteDoc function
func TestDeleteDoc(t *testing.T) {
	c := New()
	doc := document.New("/documents/1", "user", map[string]interface{}{"key": "value"})
	c.PutDoc(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/documents/1", nil), "1", &doc)

	req := httptest.NewRequest(http.MethodDelete, "/documents/1", nil)
	w := httptest.NewRecorder()

	c.DeleteDoc(w, req, "1")

	resp := w.Result()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// TestPatchDoc tests the PatchDoc function
func TestPatchDoc(t *testing.T) {
	c := New()
	doc := document.New("/documents/1", "user", map[string]interface{}{"key": "value"})
	c.PutDoc(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/documents/1", nil), "1", &doc)

	patchData := []patcher.Patch{
		{Operation: "add", Path: "/newKey", Value: "newValue"},
	}
	patchBytes, _ := json.Marshal(patchData)
	req := httptest.NewRequest(http.MethodPatch, "/documents/1", bytes.NewReader(patchBytes))
	w := httptest.NewRecorder()

	c.PatchDoc(w, req, "1", &jsonschema.Schema{}, "author")

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), `"PatchFailed":false`)
}

// TestPostDoc tests the PostDoc function
func TestPostDoc(t *testing.T) {
	c := New()
	doc := document.New("/documents", "user", map[string]interface{}{"key": "value"})
	req := httptest.NewRequest(http.MethodPost, "/documents", nil)
	w := httptest.NewRecorder()

	c.PostDoc(w, req, &doc)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Contains(t, string(body), `"uri":"/documents/`)
}
