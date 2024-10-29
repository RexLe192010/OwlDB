// Package contains a method to initialize the database
package initialize

import (
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func Initialize() (int, *jsonschema.Schema, map[string]string, error) {
	// Initialize flags
	//These are for the flags requirement
	portNum := flag.Int("p", 3318, "Port number for listening")
	schemaFlag := flag.String("s", "", "Schema path")
	tokenFlag := flag.String("t", "", "Token path")
	loggerFlag := flag.Int("l", 0, "Logger output level, -1 for debug, 1 for only errors")
	flag.Parse()

	var tokenMap map[string]string

	//A check before anything to see if the schema file exists
	if *schemaFlag == "" {
		slog.Error("Missing schema file. Specify with the -s flag", "error", errors.New("missing schema file"))
		return 0, nil, tokenMap, errors.New("missing schema file")
	}

	// Compile the schema
	schema, err := jsonschema.Compile(*schemaFlag)

	// Check for errors
	if err != nil {
		slog.Error("Invalid schema file", "error", err)
		return 0, nil, tokenMap, errors.New("invalid schema file")
	}

	// the user inputs a token file
	if *tokenFlag != "" {
		// Read in the token file
		token, err := os.ReadFile(*tokenFlag)
		if err != nil {
			slog.Error("Token file not found", "error", err)
			return 0, nil, tokenMap, errors.New("token file not found")
		}

		// Unmarshal the token file
		err = json.Unmarshal(token, &tokenMap)
		if err != nil {
			slog.Error("Error marshalling token file", "error", err)
			return 0, nil, tokenMap, errors.New("marshalling token file")
		}
	}

	// set the logger level
	if *loggerFlag == -1 {
		h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
		slog.SetDefault(slog.New(h))
	}

	// set to error only
	if *loggerFlag == 1 {
		h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})
		slog.SetDefault(slog.New(h))
	}

	return *portNum, schema, tokenMap, nil

}
