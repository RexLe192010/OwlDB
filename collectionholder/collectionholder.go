// Package collectionholder contains a struct
// and several methods for manipulating a collection of collections
package collectionholder

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/RICE-COMP318-FALL24/owldb-p1group70/errorMessage"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/interfaces"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/skiplist"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/structs"
)

type CollectionHolder struct {
	collections *skiplist.SkipList[string, interfaces.ICollection]
}

// A PutOutput stores the response to a put request.
type PutOutput struct {
	Uri string `json:"uri"` // The URI of the successful put operation.
}

// Create a new collection holder
func New() CollectionHolder {
	newSkipList := skiplist.New[string, interfaces.ICollection](skiplist.STRINGMIN, skiplist.STRINGMAX, skiplist.DEFAULT_LEVEL)
	return CollectionHolder{collections: &newSkipList}
}

// Create a new collection inside the collection holder
func (ch *CollectionHolder) PutColl(w http.ResponseWriter, r *http.Request, dbpath string, newColl interfaces.ICollection) {
	// Update and insert a new database to the databse-handler if it's not already there
	// otherwise, output error
	// Define a function to upsert a collection
	slog.Info("collectionholder PutColl: creating collection", "path", dbpath)

	upsert := func(key string, currentValue interfaces.ICollection, exists bool) (interfaces.ICollection, error) {
		if exists {
			// If the collection already exists, return an error
			return nil, errors.New("db exists")
		}
		return newColl, nil
	}

	_, err := ch.collections.Upsert(dbpath, upsert)
	slog.Info("collectionholder PutColl: upserted collection", "path", dbpath)

	if err != nil {
		slog.Error(err.Error())
		switch err.Error() {
		case "db exists":
			// 400 Bad Request
			slog.Info("collectionholder PutColl: database/collection already exists", "path", r.URL.Path)
			errorMessage.ErrorResponse(w, "Database already exists", http.StatusBadRequest)
		default:
			// 500 Internal Server Error
			slog.Error("collectionholder PutColl: error upserting collection", "error", err)
			errorMessage.ErrorResponse(w, "PUT collection error"+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// PUT is successful
	jsonResponse, err := json.Marshal(structs.PutOutput{Uri: r.URL.Path})
	if err != nil {
		// This should never happen
		slog.Error("Put: error marshalling json", "error", err)
		errorMessage.ErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	slog.Info("collectionholder PutColl: database/collection created", "path", r.URL.Path)
	w.Header().Set("Location", r.URL.Path)
	w.WriteHeader(http.StatusCreated)
	w.Write(jsonResponse)
}

// Find a collection in this collection holder.
func (ch *CollectionHolder) GetColl(resource string) (coll interfaces.ICollection, found bool) {
	slog.Debug("collectionholder GetColl: looking for collection", "path", resource)
	return ch.collections.Find(resource)
}

// Delete a collection in this collection holder.
func (ch *CollectionHolder) DeleteColl(w http.ResponseWriter, r *http.Request, dbPath string) {
	slog.Info("collectionholder DeleteColl: deleting collection", "path", dbPath, "ch", *ch)
	// request a delete on the specfied element
	coll, removed := ch.collections.Remove(dbPath)

	// handle the case where the collection is not found
	if !removed {
		slog.Info("collectionholder DeleteColl: collection not found", "path", r.URL.Path)
		errorMessage.ErrorResponse(w, "Collection not found", http.StatusNotFound)
		return
	}

	// notify subscribers
	deleteMsg := fmt.Sprintf(`"%"`, r.URL.Path)
	if subscribableColl, ok := coll.(interfaces.Subscribable); ok {
		subscribableColl.NotifySubscribersDelete(deleteMsg, "general")
	}

	slog.Info("collectionholder DeleteColl: collection deleted", "path", r.URL.Path)
	w.Header().Set("Location", r.URL.Path)
	w.WriteHeader(http.StatusNoContent)
}
