package authentication

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type test struct {
	r        *http.Request
	w        *httptest.ResponseRecorder
	expected string
	code     int
}

// test login with no username
func TestLoginNoUsername(t *testing.T) {
	testAuthenticator := NewAuthenticator()

	// Create a request with no username
	req := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader("{\"username\":\"\"}"))
	w := httptest.NewRecorder()

	// Serve the request
	testAuthenticator.ServeHTTP(w, req)
	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected error code %d got %d", http.StatusBadRequest, res.StatusCode)
	}
}

// test login with invalid username
func TestLoginInvalidFormat(t *testing.T) {
	testAuthenticator := NewAuthenticator()

	// Create a request with invalid username
	req := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader("{\"username\":}"))
	w := httptest.NewRecorder()

	testAuthenticator.ServeHTTP(w, req)
	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected error code %d got %d", http.StatusBadRequest, res.StatusCode)
	}
}

// test login with no body
func TestLogoutMissingToken(t *testing.T) {
	testAuthenticator := NewAuthenticator()

	// Create a request with no body
	req := httptest.NewRequest(http.MethodDelete, "/auth", nil)
	w := httptest.NewRecorder()

	testAuthenticator.ServeHTTP(w, req)
	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected error code %d got %d", http.StatusUnauthorized, res.StatusCode)
	}
}

// test login with invalid token
func TestLoginSuccess(t *testing.T) {
	testAuthenticator := NewAuthenticator()

	// Login to get a token
	req := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader("{\"username\":\"rexle\"}"))
	w := httptest.NewRecorder()

	testAuthenticator.ServeHTTP(w, req)
	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		// Check if the status code is 200
		t.Errorf("Expected error code %d got %d", http.StatusOK, res.StatusCode)
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		// Check if there is no error
		t.Errorf("Expected no error, got %v", err)
	}

	var tokenMap map[string]string
	err = json.Unmarshal(data, &tokenMap)
	if err != nil {
		// Check if there is no error
		t.Errorf("Expected no error, got %v", err)
	}

	token, ok := tokenMap["token"]
	if !ok {
		// Check if the token is present
		t.Errorf("Expected response to have key token, got %v", tokenMap)
	}

	// Store the token for further tests
	t.Setenv("TEST_TOKEN", token)
}

// test login with invalid token
func TestValidateTokenSuccess(t *testing.T) {
	testAuthenticator := NewAuthenticator()

	// Login to get a token
	req := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader("{\"username\":\"rexle\"}"))
	w := httptest.NewRecorder()
	testAuthenticator.ServeHTTP(w, req)
	res := w.Result()
	defer res.Body.Close()

	data, _ := io.ReadAll(res.Body)
	var tokenMap map[string]string
	json.Unmarshal(data, &tokenMap)
	token := tokenMap["token"]

	req = httptest.NewRequest(http.MethodGet, "/v1/db", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()

	username, valid := testAuthenticator.ValidateToken(w, req)
	if !valid {
		// Check if the token is valid
		t.Errorf("Expected to find valid user.")
	}
	if username != "rexle" {
		// Check if the username is correct
		t.Errorf("Expected username rexle, got %s", username)
	}
}

// test login with invalid token
func TestValidateTokenInvalid(t *testing.T) {
	testAuthenticator := NewAuthenticator()

	// Login to get a token
	req := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader("{\"username\":\"rexle\"}"))
	w := httptest.NewRecorder()
	testAuthenticator.ServeHTTP(w, req)
	res := w.Result()
	defer res.Body.Close()

	data, _ := io.ReadAll(res.Body)
	var tokenMap map[string]string
	json.Unmarshal(data, &tokenMap)
	token := tokenMap["token"]

	req = httptest.NewRequest(http.MethodGet, "/v1/db", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token+"z")
	w = httptest.NewRecorder()

	_, valid := testAuthenticator.ValidateToken(w, req)
	if valid {
		// Check if the token is invalid
		t.Errorf("Expected to find invalid user.")
	}
	res = w.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		// Check if the status code is 401
		t.Errorf("Expected error code %d got %d", http.StatusUnauthorized, res.StatusCode)
	}
}

// test login with invalid token
func TestLogoutSuccess(t *testing.T) {
	testAuthenticator := NewAuthenticator()

	// Login to get a token
	req := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader("{\"username\":\"rexle\"}"))
	w := httptest.NewRecorder()
	testAuthenticator.ServeHTTP(w, req)
	res := w.Result()
	defer res.Body.Close()

	data, _ := io.ReadAll(res.Body)
	var tokenMap map[string]string
	json.Unmarshal(data, &tokenMap)
	token := tokenMap["token"]

	req = httptest.NewRequest(http.MethodDelete, "/auth", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()

	testAuthenticator.ServeHTTP(w, req)
	res = w.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		// Check if the status code is 204
		t.Errorf("Expected error code %d, got %d", http.StatusNoContent, res.StatusCode)
	}
}

// test login with invalid token
func TestLogoutInvalidToken(t *testing.T) {
	testAuthenticator := NewAuthenticator()

	// Login to get a token
	req := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader("{\"username\":\"rexle\"}"))
	w := httptest.NewRecorder()
	testAuthenticator.ServeHTTP(w, req)
	res := w.Result()
	defer res.Body.Close()

	data, _ := io.ReadAll(res.Body)
	var tokenMap map[string]string
	json.Unmarshal(data, &tokenMap)
	token := tokenMap["token"]

	req = httptest.NewRequest(http.MethodDelete, "/auth", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token+"z")
	w = httptest.NewRecorder()

	testAuthenticator.ServeHTTP(w, req)
	res = w.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		// Check if the status code is 401
		t.Errorf("Expected error code %d got %d", http.StatusUnauthorized, res.StatusCode)
	}
}
