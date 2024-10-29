package collectionholder

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RICE-COMP318-FALL24/owldb-p1group70/collection"
	"github.com/stretchr/testify/assert"
)

// TestPutColl tests the PutColl function
func TestPutColl(t *testing.T) {
	ch := New() // Create a new collection holder
	mockColl := collection.New()
	req := httptest.NewRequest(http.MethodPut, "/collections/test", nil)
	w := httptest.NewRecorder()

	ch.PutColl(w, req, "/collections/test", &mockColl)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Contains(t, string(body), `"/collections/test"`)
}

// TestPutColl_AlreadyExists tests the PutColl function when the collection already exists
func TestPutColl_AlreadyExists(t *testing.T) {
	ch := New()
	mockColl := collection.New()
	req := httptest.NewRequest(http.MethodPut, "/collections/test", nil)
	w := httptest.NewRecorder()

	// First put to create the collection
	ch.PutColl(w, req, "/collections/test", &mockColl)

	// Second put to test already exists scenario
	w = httptest.NewRecorder()
	ch.PutColl(w, req, "/collections/test", &mockColl)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, string(body), "Database already exists")
}

// TestPutColl_InternalServerError tests the PutColl function when an internal server error occurs
func TestGetColl(t *testing.T) {
	ch := New()
	mockColl := collection.New()
	req := httptest.NewRequest(http.MethodPut, "/collections/test", nil)
	w := httptest.NewRecorder()

	// First put to create the collection
	ch.PutColl(w, req, "/collections/test", &mockColl)

	// Test GetColl
	coll, found := ch.GetColl("/collections/test")
	assert.True(t, found)
	assert.NotNil(t, coll)
}

// TestGetColl_NotFound tests the GetColl function when the collection is not found
func TestGetColl_NotFound(t *testing.T) {
	ch := New()

	// Test GetColl for non-existent collection
	coll, found := ch.GetColl("/collections/nonexistent")
	assert.False(t, found)
	assert.Nil(t, coll)
}

// TestDeleteColl tests the DeleteColl function
func TestDeleteColl(t *testing.T) {
	ch := New()
	mockColl := collection.New()
	req := httptest.NewRequest(http.MethodPut, "/collections/test", nil)
	w := httptest.NewRecorder()

	// First put to create the collection
	ch.PutColl(w, req, "/collections/test", &mockColl)

	// Test DeleteColl
	req = httptest.NewRequest(http.MethodDelete, "/collections/test", nil)
	w = httptest.NewRecorder()
	ch.DeleteColl(w, req, "/collections/test")

	resp := w.Result()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// TestDeleteColl_NotFound tests the DeleteColl function when the collection is not found
func TestDeleteColl_NotFound(t *testing.T) {
	ch := New()

	// Test DeleteColl for non-existent collection
	req := httptest.NewRequest(http.MethodDelete, "/collections/nonexistent", nil)
	w := httptest.NewRecorder()
	ch.DeleteColl(w, req, "/collections/nonexistent")

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Contains(t, string(body), "Collection not found")
}
