// This file is a skeleton for your project. You should replace this
// comment with true documentation.

package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal" //prolly not needed i think
	"syscall"

	"github.com/RICE-COMP318-FALL24/owldb-p1group70/authentication"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/collectionholder"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/errorMessage"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/handlers"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/initialize"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

func main() {
	var server *http.Server
	var port int
	var tokenMap map[string]string
	var err error
	var schema *jsonschema.Schema
	var authenticator authentication.Authenticator
	var owlDB handlers.Handler

	// Initialize flags
	port, schema, tokenMap, err = initialize.Initialize()

	authenticator = authentication.NewAuthenticator()
	database := collectionholder.New()
	owlDB = handlers.New(&database, schema, &authenticator)

	// Install handlers into the server mux
	mux := http.NewServeMux()
	mux.Handle("/v1/", &owlDB)
	mux.Handle("/auth", &authenticator)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		errorMessage.ErrorResponse(w, "Missing /v1/ or /auth in the request", 400)
	})

	// install user tokens into the authenticator
	authenticator.InstallUsers(tokenMap)

	server = &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", port),
		Handler: mux,
	}

	// Your code goes here.

	// The following code should go last and remain unchanged.
	// Note that you must actually initialize 'server' and 'port'
	// before this.  Note that the server is started below by
	// calling ListenAndServe.  You must not start the server
	// before this.

	// signal.Notify requires the channel to be buffered
	ctrlc := make(chan os.Signal, 1)
	signal.Notify(ctrlc, os.Interrupt, syscall.SIGTERM)
	go func() {
		// Wait for Ctrl-C signal
		<-ctrlc
		server.Close()
	}()

	// Start server
	slog.Info("Listening", "port", port)
	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		slog.Error("Server closed", "error", err)
	} else {
		slog.Info("Server closed", "error", err)
	}
}
