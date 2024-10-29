package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RICE-COMP318-FALL24/owldb-p1group70/collectionholder"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// A struct that carries data for running a test
type test struct {
	r        *http.Request
	w        *httptest.ResponseRecorder
	expected string
	code     int
}

// An empty struct to implement validate token.
type skeletonAuthenticator struct{}

// A simple implementation of validate token for testing.
func (skeletonAuthenticator) ValidateToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	return "rexle", true
}

func setup() (*Handler, func()) {
	// Compile the schema
	testschema, _ := jsonschema.Compile("testschema.json")

	databases := collectionholder.New()
	testhandler := New(&databases, testschema, skeletonAuthenticator{})

	return &testhandler, func() {
		// Cleanup code if needed
	}
}

func runTests(t *testing.T, testhandler *Handler, data []test) {
	for i, d := range data {
		testhandler.ServeHTTP(d.w, d.r)
		res := d.w.Result()
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Errorf("Test %d: Expected no error, got %v", i, err)
		}
		if string(body) != d.expected && d.expected != "" {
			t.Errorf("Test %d: Expected response %s got %s", i, d.expected, string(body))
		}
		if res.StatusCode != d.code {
			t.Errorf("Test %d: Expected response code %d got %d", i, d.code, res.StatusCode)
		}
	}
}

func TestPutAndGetDBsAndDocs(t *testing.T) {
	testhandler, cleanup := setup()
	defer cleanup()

	data := []test{
		{httptest.NewRequest(http.MethodPut, "/db1", nil),
			httptest.NewRecorder(),
			"", 400},
		{httptest.NewRequest(http.MethodPut, "/v1/db1", nil),
			httptest.NewRecorder(),
			"{\"uri\":\"/v1/db1\"}", 201},
		{httptest.NewRequest(http.MethodGet, "/v1/db1/", nil),
			httptest.NewRecorder(),
			"[]", 200},
		{httptest.NewRequest(http.MethodPut, "/v1/db1/doc1", strings.NewReader("{\"prop\":100}")),
			httptest.NewRecorder(),
			"{\"uri\":\"/v1/db1/doc1\"}", 201},
		{httptest.NewRequest(http.MethodPut, "/v1/db1/doc1", strings.NewReader("{\"prop\":100}")),
			httptest.NewRecorder(),
			"{\"uri\":\"/v1/db1/doc1\"}", 200},
		{httptest.NewRequest(http.MethodGet, "/v1/db1/doc2", nil),
			httptest.NewRecorder(),
			"", 404},
		{httptest.NewRequest(http.MethodGet, "/v1/db2/", nil),
			httptest.NewRecorder(),
			"", 404},
		{httptest.NewRequest(http.MethodGet, "/invalidPath", nil),
			httptest.NewRecorder(),
			"", 400},
		{httptest.NewRequest(http.MethodPut, "/invalidPath", nil),
			httptest.NewRecorder(),
			"", 400},
	}

	runTests(t, testhandler, data)
}

func TestGetDBAndDoc(t *testing.T) {
	testhandler, cleanup := setup()
	defer cleanup()

	data := []test{
		{httptest.NewRequest(http.MethodGet, "/v1/db1/doc1", nil),
			httptest.NewRecorder(),
			"", 200},
		{httptest.NewRequest(http.MethodGet, "/v1/db1/", nil),
			httptest.NewRecorder(),
			"", 200},
	}

	runTests(t, testhandler, data)
}

func TestPatchDoc(t *testing.T) {
	testhandler, cleanup := setup()
	defer cleanup()

	data := []test{
		{httptest.NewRequest(http.MethodPatch, "/v1/db1/doc1", strings.NewReader("[{\"op\":\"ObjectAdd\",\"path\":\"/a\",\"value\":100}]")),
			httptest.NewRecorder(),
			"{\"uri\":\"/v1/db1/doc1\",\"patchFailed\":false,\"message\":\"patches applied\"}", 200},
		{httptest.NewRequest(http.MethodPatch, "/v1/db1/doc1", strings.NewReader("[{\"op\":\"ObjectAdd\",\"path\":\"/b\",\"value\":100},{\"op\":\"ObjectAdd\",\"path\":\"/c\",\"value\":100}]")),
			httptest.NewRecorder(),
			"{\"uri\":\"/v1/db1/doc1\",\"patchFailed\":false,\"message\":\"patches applied\"}", 200},
		{httptest.NewRequest(http.MethodPatch, "/v1/db1/doc1", strings.NewReader("[{\"op\":\"ArrayAdd\",\"path\":\"/b\",\"value\":100},{\"op\":\"ObjectAdd\",\"path\":\"/c\",\"value\":100}]")),
			httptest.NewRecorder(),
			"", 400},
	}

	runTests(t, testhandler, data)
}

func TestPutCollectionAndPost(t *testing.T) {
	testhandler, cleanup := setup()
	defer cleanup()

	data := []test{
		{httptest.NewRequest(http.MethodPut, "/v1/db1/doc1/col/", nil),
			httptest.NewRecorder(),
			"{\"uri\":\"/v1/db1/doc1/col/\"}", 201},
		{httptest.NewRequest(http.MethodPost, "/v1/db1/", strings.NewReader("{\"prop\":100}")),
			httptest.NewRecorder(),
			"", 201},
		{httptest.NewRequest(http.MethodPost, "/v1/db1/doc1/col/", strings.NewReader("{\"prop\":100}")),
			httptest.NewRecorder(),
			"", 201},
	}

	runTests(t, testhandler, data)
}

func TestDeleteCollectionsDocsAndDBs(t *testing.T) {
	testhandler, cleanup := setup()
	defer cleanup()

	data := []test{
		{httptest.NewRequest(http.MethodDelete, "/v1/db1/doc1/col/", nil),
			httptest.NewRecorder(),
			"", 204},
		{httptest.NewRequest(http.MethodDelete, "/v1/db1/doc1/col/", nil),
			httptest.NewRecorder(),
			"", 404},
		{httptest.NewRequest(http.MethodDelete, "/v1/db1/doc1", nil),
			httptest.NewRecorder(),
			"", 204},
		{httptest.NewRequest(http.MethodDelete, "/v1/db1/doc1", nil),
			httptest.NewRecorder(),
			"", 404},
		{httptest.NewRequest(http.MethodDelete, "/v1/db1", nil),
			httptest.NewRecorder(),
			"", 204},
		{httptest.NewRequest(http.MethodDelete, "/v1/db1", nil),
			httptest.NewRecorder(),
			"", 404},
	}

	runTests(t, testhandler, data)
}
