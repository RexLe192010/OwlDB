// Package paths contains static utility methods for processing
// path name strings from requests.

package paths

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/RICE-COMP318-FALL24/owldb-p1group70/errorMessage"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/interfaces"
)

// Result codes from path resource operations.

// Indicates the type of resource obtained from a path or
// the type of error if there was an error obtaining a resource.
const (
	ERROR_BLANK_PATHNAME = -103
	ERROR_INTERNAL       = -102
	ERROR_BAD_SLASH      = -101
	ERROR_NO_VERSION     = -100
	ERROR_NO_DB          = -RESOURCE_DB
	ERROR_NO_COLL        = -RESOURCE_COLL
	ERROR_NO_DOC         = -RESOURCE_DOC

	RESOURCE_NULL       = 0
	RESOURCE_DB         = 1
	RESOURCE_COLL       = 2
	RESOURCE_DOC        = 3
	RESOURCE_DB_PUT_DEL = 4 // specifically for put and delete db w/o slash
)

// Obtain resource from the specified path "request." Start looking at the "root" collectioon holder.

// On success, returns a collection if the path leads to a collection or a database,
// or a document if the path leads to a document. Returns a result code indicating
// the type of resource returned.

// On error, returns a resource error code indicating the type of error found.
func ParsePath(request string, root interfaces.ICollectionHolder) (interfaces.ICollection, interfaces.IDocument, int) {
	slog.Info("paths ParsePath: start of parsing", "request", request)

	// Check version
	path, found := strings.CutPrefix(request, "/v1/")
	if !found {
		return nil, nil, ERROR_NO_VERSION
	}

	slog.Info("paths ParsePath: path after version", "path", path)

	resources := strings.Split(path, "/")
	slog.Info("paths ParsePath: resources from splitting path", "resources", resources)

	// Identify resource type
	finalRes := RESOURCE_NULL

	// Handle errors
	if len(resources) == 0 {
		// This case is redundant because strings.Split will never return a slice with length 0
		return nil, nil, ERROR_BAD_SLASH
	} else if len(resources) > 1 && len(resources)%2 == 1 {
		// Slash used for a document or end in a collection
		// /v1/db/doc/ or /v1/db/doc/coll
		return nil, nil, ERROR_BAD_SLASH
	}

	// Identify the final resource
	// If the last element ends with a slash, then it must be a collection
	if len(resources) == 1 {
		// /v1/db
		finalRes = RESOURCE_DB_PUT_DEL
	} else if len(resources) == 2 && resources[1] == "" {
		// /v1/db/
		finalRes = RESOURCE_DB
	} else if len(resources) > 2 && resources[len(resources)-1] == "" {
		finalRes = RESOURCE_COLL
	} else {
		finalRes = RESOURCE_DOC
	}

	slog.Debug("paths ParsePath: resCode", "finalRes", finalRes)

	// Iterate over the path
	var lastColl interfaces.ICollection
	var lastDoc interfaces.IDocument

	for i, resource := range resources {
		slog.Info("paths ParsePath: iterate over path", "index", i, "resource", resource, "resources", resources)
		// Handle slash
		if resource == "" {
			if i != len(resources)-1 {
				// invalid resource name
				return nil, nil, ERROR_BLANK_PATHNAME
			}

			// Blank database put/delete
			if i == 0 {
				return nil, nil, ERROR_BAD_SLASH
			}

			// Error checking
			if lastColl == nil {
				slog.Info("paths ParsePath: Returning NIL collection")
				slog.Error("paths ParsePath: Returning NIL collection")
				return nil, nil, ERROR_INTERNAL
			}

			// Return a database or collection
			return lastColl, nil, finalRes
		}

		// Change behavior based on the interation
		if i == 0 {
			// Database
			slog.Info("paths ParsePath: start from database", "resource", resource)
			slog.Info("paths ParsePath: root", "root", &root)
			lastColl, found = root.GetColl(resource)
		} else if i%2 == 1 {
			// Document
			lastDoc, found = lastColl.FindDoc(resource)
		} else if i > 0 && i%2 == 0 {
			// Collection
			collHolder, hasCollection := interface{}(lastDoc).(interfaces.ICollectionHolder)
			if hasCollection {
				lastColl, found = collHolder.GetColl(resource)
			}
		}
		if !found {
			slog.Info("Resource not found", "index", i, "resource", resource, "resources", resources)
			return nil, nil, -finalRes
		}
	}

	// End without a slash --- either a database-put-delete or a document
	if finalRes == RESOURCE_DB_PUT_DEL {
		if lastColl == nil {
			slog.Error("ParsePath: Returning NIL database")
			return nil, nil, ERROR_INTERNAL
		}
		return lastColl, nil, finalRes
	} else if finalRes == RESOURCE_DOC {
		if lastDoc == nil {
			slog.Error("ParsePath: Returning NIL document")
			return nil, nil, ERROR_INTERNAL
		}
		return nil, lastDoc, finalRes
	} else {
		return nil, nil, ERROR_INTERNAL
	}

}

// Obtain the parent resource from the specified path "request."
// Returns the truncated request, the resource name, and a result code.
func GetParentResource(request string) (truncatedRequest string, parentResource string, resourceCode int) {
	// check version
	path, found := strings.CutPrefix(request, "/v1/")
	if !found {
		return "", "", ERROR_NO_VERSION
	}

	resources := strings.Split(path, "/")
	slog.Info("paths GetParentResource: resources from splitting path", "resources", resources)

	// identify resource type
	finalRes := RESOURCE_NULL

	// handle errors
	if len(resources) == 0 {
		// /v1/
		return "", "", ERROR_BAD_SLASH
	} else if len(resources) == 1 {
		// /v1/db
		return "", resources[0], RESOURCE_DB_PUT_DEL
	} else if len(resources) == 2 && resources[1] == "" {
		// /v1/db/
		return "", resources[0], RESOURCE_DB
	} else if len(resources)%2 == 1 {
		// /v1/db/doc/ or /v1/db/doc/coll
		return "", "", ERROR_BAD_SLASH
	}

	// identify the final resource as a collection or database

	// if the last element ends with a slash, then it must be a collection
	lastIndex := strings.LastIndex(request, "/")
	resName := request[lastIndex+1:]
	if resources[len(resources)-1] == "" {
		// collection -- truncate by 2
		// go to a document
		lastIndex2 := strings.LastIndex(request[:lastIndex-1], "/")
		finalRes = RESOURCE_COLL
		resName = request[lastIndex2+1 : lastIndex]
		request = request[:lastIndex2]
	} else {
		// document -- truncate by 1
		finalRes = RESOURCE_DOC
		request = request[:lastIndex+1]
	}

	slog.Info("paths GetParentResource: truncated resource path", "finalRes", finalRes, "request", request, "resName", resName)
	return request, resName, finalRes
}

func HandlePathError(w http.ResponseWriter, r *http.Request, errCode int) {
	switch errCode {
	case ERROR_BAD_SLASH:
		slog.Error("Invalid path: Bad slash", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid path: Bad slash in %s", r.URL.Path)
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	case ERROR_NO_VERSION:
		slog.Error("Invalid path: No version", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid path: No version in %s", r.URL.Path)
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	case ERROR_NO_DB:
		slog.Error("Invalid path: No database", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid path: No database in %s", r.URL.Path)
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	case ERROR_NO_COLL:
		slog.Error("Invalid path: No collection", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid path: No collection in %s", r.URL.Path)
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	case ERROR_NO_DOC:
		slog.Error("Invalid path: No document", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid path: No document in %s", r.URL.Path)
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	case RESOURCE_DB:
		slog.Info("Invalid database resource for request", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid database resource for request in %s", r.URL.Path)
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	case RESOURCE_COLL:
		slog.Info("Invalid collection resource for request", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid collection resource for request in %s", r.URL.Path)
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	case RESOURCE_DOC:
		slog.Info("Invalid document resource for request", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid document resource for request in %s", r.URL.Path)
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	case RESOURCE_DB_PUT_DEL:
		slog.Info("Invalid database put/delete resource for request", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid database put/delete resource for request in %s", r.URL.Path)
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	case ERROR_BLANK_PATHNAME:
		slog.Error("Invalid path: Blank pathname", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid path: Blank pathname in %s", r.URL.Path)
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	default:
		slog.Error("Invalid path: Internal error", "path", r.URL.Path)
		msg := fmt.Sprintf("Invalid path: Internal error in %s", r.URL.Path)
		errorMessage.ErrorResponse(w, msg, http.StatusInternalServerError)
	}
}

// Take a path with a /v1/db/<path> and remove the /v1/db/.
func GetRelativePathNonDB(path string) string {
	splitpath := strings.SplitAfterN(path, "/", 4)
	return "/" + splitpath[3]
}

// Take a path with a /v1/db and remove the /v1.
func GetRelativePathDB(path string) string {
	trimmedpath := strings.TrimPrefix(path, "/v1")
	return trimmedpath
}
