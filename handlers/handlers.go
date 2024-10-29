package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/RICE-COMP318-FALL24/owldb-p1group70/collection"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/document"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/errorMessage"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/interfaces"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/paths"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

type Handler struct {
	DB            interfaces.ICollectionHolder // The database service
	schema        *jsonschema.Schema           // The schema for validation
	authenticator interfaces.Authenticator     // The authentication service
}

// Create a new handler
func New(db interfaces.ICollectionHolder, schema *jsonschema.Schema, authenticator interfaces.Authenticator) Handler {
	return Handler{db, schema, authenticator}
}

// The server implements the "handler" interface,
// It will recieve requests from the user and delegate them to the proper methods.
func (d *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slog.Debug("handlers ServeHTTP: Request being handled", "method", r.Method, "path", r.URL.Path)

	// Set headers of response
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Check if user is valid.
	if r.Method == http.MethodOptions {
		slog.Debug("handlers ServeHTTP: User requested OPTIONS", "method", r.Method)
		Options(w, r)
	} else {
		username, valid := d.authenticator.ValidateToken(w, r)
		if valid {
			if r.URL.Path == "/v1/subscribe" && r.Method == http.MethodGet {
				d.handleSubscribe(w, r)
				return
			}

			switch r.Method {
			case http.MethodGet:
				d.get(w, r)
			case http.MethodPut:
				d.put(w, r, username)
			case http.MethodDelete:
				d.delete(w, r)
			case http.MethodPatch:
				d.patch(w, r, username)
			case http.MethodPost:
				d.post(w, r, username)
			default:
				// If user used method we do not support.
				slog.Info("handlers ServeHTTP: user used unsupported method", "method", r.Method)
				msg := fmt.Sprintf("unsupported method: %s", r.Method)
				errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
			}

		}
	}
}

// Top-level function to perform the HTTP GET request
// getDB, getColl, and getDoc are implemented in their respective files.
func (d *Handler) get(w http.ResponseWriter, r *http.Request) {
	coll, doc, resCode := paths.ParsePath(r.URL.Path, d.DB)
	switch resCode {
	case paths.RESOURCE_DB:
		// GET collection of documents from database
		d.getColl(w, r, coll)
	case paths.RESOURCE_COLL:
		// GET document from collection
		coll.GetDoc(w, r)
	case paths.RESOURCE_DOC:
		// GET collection from document
		doc.GetDoc(w, r)
	default:
		paths.HandlePathError(w, r, resCode)
	}
}

// Top-level function to perform the HTTP PUT request
func (d *Handler) put(w http.ResponseWriter, r *http.Request, username string) {
	slog.Debug("handlers put: top-level put handling")
	// Obtain the parent resource from the path
	newRequest, newRequestName, resCode := paths.GetParentResource(r.URL.Path)
	slog.Debug("handlers put: parent resource obtained", "newRequest", newRequest, "newRequestName", newRequestName, "resCode", resCode)

	// Put request handling based on the resource type
	if resCode == paths.RESOURCE_DB_PUT_DEL {
		dbPath := newRequestName //expecting string in putDB so it hink it has to be this?
		d.putDB(w, r, dbPath)
		return
	} else if resCode == paths.RESOURCE_DB {
		slog.Info("handlers put: bad syntax, user is trying to PUT database", "path", r.URL.Path)
		msg := fmt.Sprintf("Bad syntax: user is trying to PUT database in %s", r.URL.Path)
		errorMessage.ErrorResponse(w, msg, http.StatusBadRequest)
	} else if resCode < 0 {
		paths.HandlePathError(w, r, resCode)
		return
	}

	// Handle collection and document PUT requests
	coll, doc, resCode := paths.ParsePath(newRequest, d.DB)
	slog.Info("handlers put: resource from ParsePath", "coll", coll, "doc", doc, "resCode", resCode)
	switch resCode {
	case paths.RESOURCE_DB:
		// PUT document in database
		doc, err := d.createDoc(w, r, username)
		if err != nil {
			return
		}
		coll.PutDoc(w, r, newRequestName, &doc)
	case paths.RESOURCE_COLL:
		// PUT document in collection
		doc, err := d.createDoc(w, r, username)
		if err != nil {
			return
		}
		coll.PutDoc(w, r, newRequestName, &doc)
	case paths.RESOURCE_DOC:
		// PUT collection in document
		coll := collection.New() // Create a new collection

		// convert the original doc to collHolder
		collHolder, hasCollection := interface{}(doc).(interfaces.ICollectionHolder)
		slog.Info("handlers put: document to collectionholder conversion results", "doc", doc, "hasCollection", hasCollection)
		if hasCollection {
			slog.Info("handlers put: document converted to a collectionholder", "doc", doc)
			collHolder.PutColl(w, r, newRequestName, &coll)
		} else {
			paths.HandlePathError(w, r, resCode)
		}
	default:
		paths.HandlePathError(w, r, resCode)
	}
}

// Top-level function to perform the HTTP DELETE request
func (d *Handler) delete(w http.ResponseWriter, r *http.Request) {
	slog.Debug("handlers delete: top-level delete handling")
	// Obtain the parent resource from the path
	newRequest, newName, resCode := paths.GetParentResource(r.URL.Path)
	slog.Debug("handlers delete: parent resource obtained", "newRequest", newRequest, "newName", newName, "resCode", resCode)

	// Delete the database
	if resCode == paths.RESOURCE_DB_PUT_DEL {
		d.deleteDB(w, r, newName)
		return
	} else if resCode == paths.RESOURCE_DB || resCode < 0 {
		paths.HandlePathError(w, r, resCode)
		return
	}

	// Handle collection and document DELETE requests
	coll, doc, resCode := paths.ParsePath(newRequest, d.DB)
	switch resCode {
	case paths.RESOURCE_DB_PUT_DEL:
		d.deleteDB(w, r, newName)
		return
	case paths.RESOURCE_DB:
		// Delete document from database
		coll.DeleteDoc(w, r, newName)
	case paths.RESOURCE_COLL:
		// Delete a document from a collection
		coll.DeleteDoc(w, r, newName)
	case paths.RESOURCE_DOC:
		// Delete a collection from a document
		collHolder, hasColleciton := interface{}(doc).(interfaces.ICollectionHolder)
		if hasColleciton {
			slog.Info("handlers delete: document converted to a collectionholder", "doc", doc)
			collHolder.DeleteColl(w, r, newName)
		} else {
			paths.HandlePathError(w, r, resCode)
		}

	default:
		paths.HandlePathError(w, r, resCode)
	}
}

// top-level function to perform the HTTP POST request
// handle POST requests for databases, collections
// on success, add a new document with a random name to the collection or database
func (d *Handler) post(w http.ResponseWriter, r *http.Request, username string) {
	// action fork based on the resource type
	coll, _, resCode := paths.ParsePath(r.URL.Path, d.DB)
	switch resCode {
	case paths.RESOURCE_DB:
		// POST document in database
		d.postDoc(w, r, coll, username)
	case paths.RESOURCE_COLL:
		// POST document in collection
		doc, err := d.createDoc(w, r, username)
		if err != nil {
			// handled in createDoc
			return
		}
		coll.PostDoc(w, r, &doc)
	default:
		paths.HandlePathError(w, r, resCode)
	}
}

// top-level function to perform the HTTP PATCH request
// handle PATCH requests for databases, collections
// on success, apply the patch to the document
func (d *Handler) patch(w http.ResponseWriter, r *http.Request, username string) {
	// patch requires the parent resource
	newRequest, newName, resCode := paths.GetParentResource(r.URL.Path)
	if resCode < 0 {
		paths.HandlePathError(w, r, resCode)
		return
	}

	coll, _, resCode := paths.ParsePath(newRequest, d.DB)
	switch resCode {
	case paths.RESOURCE_DB:
		coll.PatchDoc(w, r, newName, d.schema, username)
	case paths.RESOURCE_COLL:
		coll.PatchDoc(w, r, newName, d.schema, username)
	default:
		paths.HandlePathError(w, r, resCode)
	}
}

// Handle OPTIONS requests
func Options(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "GET,PUT,POST,PATCH,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,PUT,POST,PATCH,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "accept,Content-Type,Authorization")
	w.WriteHeader(http.StatusOK)
}

// specific handler for GET database (get a collection of documents from a database)
func (d *Handler) getColl(w http.ResponseWriter, r *http.Request, coll interfaces.ICollection) {
	coll.GetDoc(w, r)
}

// Specific handler for PUT database (create a new database)
func (d *Handler) putDB(w http.ResponseWriter, r *http.Request, dbpath string) {
	// Same behavior as collection for now
	coll := collection.New()
	d.DB.PutColl(w, r, dbpath, &coll)
}

// Specific handler for POST database (create a new document in a database)
func (d *Handler) postDoc(w http.ResponseWriter, r *http.Request, coll interfaces.ICollection, username string) {
	doc, err := d.createDoc(w, r, username)
	if err != nil {
		// handled in createDoc
		return
	}
	coll.PostDoc(w, r, &doc)
}

// delete a top-level database
func (d *Handler) deleteDB(w http.ResponseWriter, r *http.Request, newName string) {
	// Same behavior as collection for now
	d.DB.DeleteColl(w, r, newName)
}

// Create a new document to insert into a collection
func (d *Handler) createDoc(w http.ResponseWriter, r *http.Request, name string) (document.Document, error) {
	var zero document.Document
	// Read body of requests
	desc, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		slog.Error("handlers createDoc: error reading the document request body", "error", err)
		errorMessage.ErrorResponse(w, "invalid document format", http.StatusBadRequest)
		return zero, err
	}

	// Read Body data
	var docBody map[string]interface{}
	err = json.Unmarshal(desc, &docBody)
	if err != nil {
		slog.Error("handlers createDoc: error unmarshaling Post document request", "error", err)
		errorMessage.ErrorResponse(w, "invalid Post document format", http.StatusBadRequest)
		return zero, err
	}

	// Validate against schema
	err = d.schema.Validate(docBody)
	if err != nil {
		slog.Error("handlers createDoc: document did not conform to schema", "error", err)
		errorMessage.ErrorResponse(w, "document did not conform to schema", http.StatusBadRequest)
		return zero, err
	}

	return document.New(paths.GetRelativePathNonDB(r.URL.Path), name, docBody), nil
}

// Handle the subscribe request
func (d *Handler) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	// Get the query parameters
	collectionName := r.URL.Query().Get("collection")
	intervalStart := r.URL.Query().Get("start")
	intervalEnd := r.URL.Query().Get("end")

	if collectionName == "" || intervalStart == "" || intervalEnd == "" {
		// If the required query parameters are missing
		http.Error(w, "The required query parameters are missing: collection, start, end", http.StatusBadRequest)
		return
	}

	coll, exists := d.DB.GetColl(collectionName)
	if !exists {
		http.Error(w, fmt.Sprintf("Collection '%s' not found", collectionName), http.StatusNotFound)
		return
	}

	err := coll.Subscribe(w, r, intervalStart, intervalEnd)
	if err != nil {
		http.Error(w, "Subscription failed: "+err.Error(), http.StatusBadRequest)
		return
	}
}
