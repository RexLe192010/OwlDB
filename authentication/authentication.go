// Package authentication has structs and methods for enabling login and logout functionality per the owlDB specifications.
// Implement the handler interface, expect input urls to start with "/auth."
package authentication

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/RICE-COMP318-FALL24/owldb-p1group70/errorMessage"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/handlers"
)

// A concurrency-safe map to store for user sessions
type Authenticator struct {
	sessions *sync.Map
	users    map[string]string //mapping username to pw
}

// A struct to represent a user session
type sessionInfo struct {
	username   string
	expiration time.Time
}

// Initialize a new Authenticator
func NewAuthenticator() Authenticator {
	return Authenticator{
		sessions: &sync.Map{},
		users:    make(map[string]string), //initializing users map
	}
}

// Install a map from username to login tokens in the Authenticator.
// The users' sessions will last for 24 hours.
func (a *Authenticator) InstallUsers(users map[string]string) {
	// Iterate over the users map and store each user and their token in the sessions map
	for user, token := range users {
		a.sessions.Store(token, sessionInfo{user, time.Now().Add(24 * time.Hour)})
		a.users[user] = token
	}
}

// ServeHTTP implements the http.Handler interface for the Authenticator
func (a *Authenticator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		a.login(w, r)
	case http.MethodDelete:
		a.logout(w, r)
	case http.MethodOptions:
		handlers.Options(w, r)
	default:
		// if user used method we do not support
		slog.Info("User used unsupported method", "method", r.Method)
		msg := fmt.Sprintf("unsupported method: %s", r.Method)
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	}
}

// Generate a pseudo-random token for a user
func generateToken() (string, error) {
	// Generate a random 32-byte token
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		// Handle error
		return "", err
	}
	// Encode the token to a URL-safe base64 string
	token := base64.URLEncoding.EncodeToString(b)
	return token, nil
}

// Validate a token and return the associated username if valid
func (a *Authenticator) ValidateToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	w.Header().Set("Content-Type", "application/json")

	// Check if the token is missing
	inputAuth := r.Header.Get("Authorization")
	components := strings.SplitN(inputAuth, " ", 2)
	slog.Info("Validating request", "components", components)

	if len(components) != 2 || strings.ToLower(components[0]) != "bearer" || components[1] == "" {
		// Missing, or the invalid token format
		slog.Info("ValidateToken: missing or invalid bearer token format", "token", inputAuth)
		errorMessage.ErrorResponse(w, "missing or invalid bearer token format", http.StatusUnauthorized)
		return "", false
	}

	token := components[1]

	// Validate the token and check the expiration date
	userInfo, ok := a.sessions.Load(token)
	if ok {
		if !userInfo.(sessionInfo).expiration.After(time.Now()) {
			// Token has expired
			slog.Info("ValidateToken: token expired", "token", token)
			errorMessage.ErrorResponse(w, "token expired", http.StatusUnauthorized)
			return "", false
		} else {
			// Token is valid
			slog.Info("ValidateToken: token valid", "token", token)
			return userInfo.(sessionInfo).username, true
		}
	} else {
		// Token not found
		slog.Info("ValidateToken: token not found", "token", token)
		errorMessage.ErrorResponse(w, "token not found", http.StatusUnauthorized)
		return "", false
	}
}

// Login the user and return a token
func (a *Authenticator) login(w http.ResponseWriter, r *http.Request) {
	// Set the header for JSON response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Read the request body
	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		slog.Error("Login: error reading request body", "error", err)
		errorMessage.ErrorResponse(w, "error reading request body", http.StatusBadRequest)
		return
	}

	// Parse the request body to get username and password
	var credentials map[string]string
	if err := json.Unmarshal(body, &credentials); err != nil {
		slog.Error("Login: error unmarshalling request body", "error", err)
		errorMessage.ErrorResponse(w, "error unmarshalling request body", http.StatusBadRequest)
		return
	}

	username := credentials["username"]
	if username == "" {
		slog.Error("Login: missing username")
		errorMessage.ErrorResponse(w, "missing username", http.StatusBadRequest)
		return
	}

	// Generate a token for the user
	token, err := generateToken()
	if err != nil {
		// This should not happen, but handle it just in case
		slog.Error("Login: error generating token", "error", err)
		errorMessage.ErrorResponse(w, "error generating token", http.StatusInternalServerError)
		return
	}

	// Add the user session to the Authenticator
	a.sessions.Store(token, sessionInfo{username, time.Now().Add(24 * time.Hour)})

	// Return the token in the response
	jsonToken, err := json.Marshal(map[string]string{"token": token})
	if err != nil {
		slog.Error("Login: error marshalling token response", "error", err)
		errorMessage.ErrorResponse(w, "error marshalling token response", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(jsonToken)
	slog.Info("Login: successful", "username", username)
}

// Logout the user by invalidating the token
func (a *Authenticator) logout(w http.ResponseWriter, r *http.Request) {
	// Set the header for JSON response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	slog.Info("Logout: request received")

	_, isValidToken := a.ValidateToken(w, r)
	if isValidToken {
		// If the token is valid, remove it from the sessions
		inputAuth := r.Header.Get("Authorization")
		components := strings.SplitN(inputAuth, " ", 2)
		token := components[1]
		a.sessions.Delete(token)
		slog.Info("Logout: successful", "token", token)
		w.WriteHeader(http.StatusNoContent)
		w.Write([]byte(`{"message": "Logout successful"}`))
	} else {
		slog.Info("Logout: invalid token")
	}
}
