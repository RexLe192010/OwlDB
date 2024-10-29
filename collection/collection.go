// Package Collection contains a struct and methods for implementing collections.
// The Collection struct implements the ICollection interface.
// The Collection struct contains a skiplist of documents and a subscriber manager.
// The Collection struct has methods for handling GET, PUT, DELETE, PATCH, and POST requests.
package collection

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/RICE-COMP318-FALL24/owldb-p1group70/errorMessage"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/interfaces"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/patcher"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/skiplist"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/structs"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/subscribe"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

type Collection struct {
	documents         *skiplist.SkipList[string, interfaces.IDocument] // The documents in this collection
	subscriberManager *subscribe.SubscriberManager                     // The subscriber manager for this collection
}

// Create a new collection
func New() Collection {
	// the skiplist containing the documents
	newSkipList := skiplist.New[string, interfaces.IDocument](skiplist.STRINGMIN, skiplist.STRINGMAX, skiplist.DEFAULT_LEVEL)

	// the subscriber manager
	subscriberManager := subscribe.NewSubscriberManager()

	return Collection{&newSkipList, subscriberManager}
}

// Handle a get request pointing to this collection
func (c *Collection) GetDoc(w http.ResponseWriter, r *http.Request) {
	// Get queries from the URL, as well as the mode and interval
	queries := r.URL.Query()
	mode := queries.Get("mode")
	interval := getInterval(queries.Get("interval"))

	if len(interval) < 2 {
		errorMessage.ErrorResponse(w, "Invalid interval parameters", http.StatusBadRequest)
		return
	}

	if mode == "subscribe" {
		intervalStart := interval[0]
		intervalEnd := interval[1]

		if intervalStart == "" || intervalEnd == "" {
			errorMessage.ErrorResponse(w, "Missing interval params", http.StatusBadRequest)
			return
		}

		//let me comment it out for now
		//subscription stuff
		//err := c.Subscribe(w, r, intervalStart, intervalEnd)
		//if err != nil {
		//	errorMessage.ErrorResponse(w, "Subscription failed: "+err.Error(), http.StatusInternalServerError)
		//}
		return
	}

	// The final output documents
	docOutput := make([]interface{}, 0)

	// query on the collection
	pairs, err := c.documents.Query(r.Context(), interval[0], interval[1])

	// skiplist query version
	if err != nil {
		slog.Error("collection GetDoc: error querying collection", "error", err)
		errorMessage.ErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	for _, pair := range pairs {
		// Collect the document output
		docOutput = append(docOutput, pair.Value.GetRawDoc())
	}

	jsonResponse, err := json.Marshal(docOutput)
	if err != nil {
		// This should never happen
		slog.Error("collection GetDoc: error marshalling json", "error", err)
		errorMessage.ErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// GET success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)

}

// Handle a put request pointing to this collection
func (c *Collection) PutDoc(w http.ResponseWriter, r *http.Request, path string, newDoc interfaces.IDocument) {
	// Marshal
	jsonResponse, err := json.Marshal(structs.PutOutput{Uri: r.URL.Path})
	if err != nil {
		// This should never happen
		slog.Error("collection PutDoc: error marshalling json", "error", err)
		errorMessage.ErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Conditional put on timestamp
	timeStampString := r.URL.Query().Get("timestamp")
	var timeStamp int64 = -1

	if timeStampString != "" {
		val, err := strconv.Atoi(timeStampString)
		if err != nil {
			slog.Error("collection PutDoc: bad timestamp", "error", err)
			errorMessage.ErrorResponse(w, "Bad timestamp", http.StatusBadRequest)
		}
		timeStamp = int64(val)
	}

	// upsert document; update if found, create if not
	docUpsert := func(key string, currentValue interfaces.IDocument, exists bool) (interfaces.IDocument, error) {
		if exists { // the document exists, update it
			docMeta, hasMeta := interface{}(newDoc).(interfaces.HasMetadata)
			docOverwrite, hasOverwrite := interface{}(newDoc).(interfaces.Overwriteable)

			if !hasMeta || !hasOverwrite {
				return nil, errors.New("Bad overwrite")
			}

			// conditional put on timestamp
			matchOld := timeStamp == docMeta.GetLastModified()
			if timeStamp != -1 && !matchOld {
				return nil, errors.New("Bad timestamp")
			}

			// modify the metadata
			docOverwrite.OverwriteBody(newDoc.GetJSONDoc(), docMeta.GetOriginalAuthor())

			_, err := json.Marshal(currentValue.GetRawDoc())
			if err != nil {
				errorMessage.ErrorResponse(w, "internal server error", http.StatusInternalServerError)
				return nil, err
			}

			return currentValue, nil
		} else {
			// create the document
			_, err := json.Marshal(newDoc.GetRawDoc())
			if err != nil {
				errorMessage.ErrorResponse(w, "collection PutDoc: error marshalling", http.StatusInternalServerError)
				return nil, errors.New("marshalling error")
			}

			return newDoc, nil
		}
	}

	updated, err := c.documents.Upsert(path, docUpsert)
	if err != nil {
		switch err.Error() {
		case "Bad timestamp":
			slog.Error(err.Error())
			errorMessage.ErrorResponse(w, "collection PutDoc: bad timestamp", http.StatusBadRequest)
		case "Bad overwrite":
			slog.Error(err.Error())
			errorMessage.ErrorResponse(w, "collection PutDoc: bad overwrite", http.StatusBadRequest)
		default:
			slog.Error(err.Error())
			errorMessage.ErrorResponse(w, "collection PutDoc: error"+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// PUT success
	w.Header().Set("Location", r.URL.Path)
	slog.Info("collection PutDoc: document created", "path", r.URL.Path)
	// if updated {
	if updated {
		slog.Info("collection PutDoc: document updated", "path", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	} else {
		slog.Info("collection PutDoc: document created", "path", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}

	// notify subscribers
	updateMsg, err := createUpdateMessage("update", newDoc)
	if err == nil {
		intervalVal := determineInterval(newDoc)
		c.NotifySubscribersUpdate(updateMsg, intervalVal)
	}

	w.Write(jsonResponse)
}

func (c *Collection) DeleteDoc(w http.ResponseWriter, r *http.Request, docPath string) {
	// request to delete a document
	_, removed := c.documents.Remove(docPath)
	if !removed {
		// document not found
		slog.Info("collection DeleteDoc: document not found", "path", docPath)
		errorMessage.ErrorResponse(w, "Document not found", http.StatusNotFound)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// notify subscribers
	deleteMsg, err := createDeleteMessage(docPath)
	if err == nil {
		intervalVal := determineIntervalForDeletion(docPath)
		c.NotifySubscribersDelete(deleteMsg, intervalVal)
	}

	slog.Info("collection DeleteDoc: document deleted", "path", docPath)
	w.Header().Set("Location", r.URL.Path)
	w.WriteHeader(http.StatusNoContent)
}

// Handle a patch request to a document in this collection
func (c *Collection) PatchDoc(w http.ResponseWriter, r *http.Request, docPath string, schema *jsonschema.Schema, name string) {
	// retrieve the document
	doc, found := c.documents.Find(docPath)

	// handle the case where the document is not found
	if !found {
		slog.Info("collection PatchDoc: document not found", "path", docPath)
		errorMessage.ErrorResponse(w, "Document not found", http.StatusNotFound)
		return
	}

	var patchData []patcher.Patch

	// read the request
	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		slog.Error("collection PatchDoc: error reading the request body", "error", err)
		errorMessage.ErrorResponse(w, "Invalid patch format", http.StatusBadRequest)
		return
	}

	// unmarshal the body
	err = json.Unmarshal(body, &patchData)
	if err != nil {
		slog.Error("collection PatchDoc: error unmarshalling patch request", "error", err)
		errorMessage.ErrorResponse(w, "Invalid patch format", http.StatusBadRequest)
		return
	}

	// check if patchable
	patcher, canPatch := interface{}(doc).(interfaces.Patchable)
	if !canPatch {
		slog.Error("collection PatchDoc: document cannot be patched", "error", err)
		errorMessage.ErrorResponse(w, "Document cant be patched", http.StatusBadRequest)
		return
	}

	// apply the patch to the document
	patchResponse, newDoc := patcher.ApplyPatches(patchData, schema)
	patchResponse.Uri = r.URL.Path

	// marshal the response
	jsonResponse, err := json.Marshal(patchResponse)
	if err != nil {
		slog.Error("collection PatchDoc: error marshalling json", "error", err)
		errorMessage.ErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// patch success
	if !patchResponse.PatchFailed {
		// modify the metadata
		patcher.OverwriteBody(newDoc, name)

		_, err := json.Marshal(doc.GetRawDoc())
		if err != nil {
			errorMessage.ErrorResponse(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// upsert the document
		patchUpsert := func(key string, currentValue interfaces.IDocument, exists bool) (interfaces.IDocument, error) {
			if exists {
				// delete the children of the document
				return doc, nil
			} else {
				// what we expect to happen
				return nil, errors.New("Document not found")
			}
		}

		updated, err := c.documents.Upsert(docPath, patchUpsert)
		if !updated {
			// This should never happen
			slog.Error("collection PatchDoc: error upserting document", "error", err)
			errorMessage.ErrorResponse(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// notify subscribers
		updateMsg, err := createUpdateMessage("update", doc)
		if err == nil {
			intervalVal := determineInterval(doc)
			c.NotifySubscribersUpdate(updateMsg, intervalVal)
		}

		// success
		slog.Info("collection PatchDoc: document patched", "path", docPath)
		w.WriteHeader(http.StatusOK)
		w.Header().Set("location", r.URL.Path)
	} else {
		// failure
		slog.Info("collection PatchDoc: patch failed", "path", docPath)
		w.WriteHeader(http.StatusBadRequest)
	}

	w.Write(jsonResponse)
}

// Handle a post request to this collection
func (c *Collection) PostDoc(w http.ResponseWriter, r *http.Request, newDoc interfaces.IDocument) {
	slog.Info("collection PostDoc: posting document", "path", r.URL.Path)

	postDoc, canPost := interface{}(newDoc).(interfaces.Postable)
	if !canPost {
		slog.Error("collection PostDoc: document cant be posted")
		errorMessage.ErrorResponse(w, "Document cant be posted", http.StatusBadRequest)
		return
	}

	// upsert the document
	docUpsert := func(key string, currentValue interfaces.IDocument, exists bool) (interfaces.IDocument, error) {
		if exists {
			return nil, errors.New("Document exists")
		} else {
			_, err := json.Marshal(newDoc.GetRawDoc())
			if err != nil {
				errorMessage.ErrorResponse(w, "Internal server error", http.StatusInternalServerError)
				return nil, errors.New("Marshalling error")
			}

			// concatenate the key
			postDoc.ConcatPath(key)
			return newDoc, nil
		}
	}

	var path string
	for {
		// same as authentication generateToken
		// generate a 16-byte or 128-bit random token
		token := make([]byte, 16)

		// fill the slice with random bytes
		_, err := rand.Read(token)
		if err != nil {
			slog.Error("collection PostDoc: error generating token", "error", err)
			errorMessage.ErrorResponse(w, "Could not generate random token", http.StatusInternalServerError)
			return
		}

		// convert the token to a hexadecimal string
		randomName := hex.EncodeToString(token)
		_, upsertError := c.documents.Upsert(randomName, docUpsert)
		slog.Info("collection PostDoc: Upsert complete", "randomName", randomName)
		if upsertError != nil {
			switch upsertError.Error() {
			case "Document exists":
				slog.Info("collection PostDoc: document exists", "randomName", randomName)
			default:
				slog.Error(upsertError.Error())
				errorMessage.ErrorResponse(w, "collection PostDoc: error"+upsertError.Error(), http.StatusInternalServerError)
			}

			// if exists, try again
			continue
		}

		// no error, success
		path = randomName
		break
	}

	// marshal the response
	jsonResponse, err := json.Marshal(structs.PutOutput{Uri: r.URL.Path + path})
	if err != nil {
		slog.Error("PostDoc: error marshalling json", "error", err)
		errorMessage.ErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// notify subscribers
	updateMsg, err := createUpdateMessage("create", newDoc)
	if err == nil {
		intervalVal := determineInterval(newDoc)
		c.NotifySubscribersUpdate(updateMsg, intervalVal)
	}

	// success
	slog.Info("PostDoc: document created", "path", r.URL.Path+path)
	w.Header().Set("Location", r.URL.Path+path)
	w.WriteHeader(http.StatusCreated)
	w.Write(jsonResponse)
}

// Find a document in this collection for other methods.
func (c *Collection) FindDoc(resource string) (interfaces.IDocument, bool) {
	return c.documents.Find(resource)
}

// Convert a string representing string intervals into the elements inside the interval
func getInterval(intervalStr string) [2]string {
	interval := [2]string{skiplist.STRINGMIN, skiplist.STRINGMAX}
	// Must be in array form
	if !(len(intervalStr) > 2 && intervalStr[0] == '[' && intervalStr[len(intervalStr)-1] == ']') {
		slog.Info("collection getInterval: Bad interval, non-array", "interval", intervalStr)
		return interval
	}

	// Get rid of array surrounders and split
	intervalStr = intervalStr[1 : len(intervalStr)-1]
	procArr := strings.Split(intervalStr, ",")

	if len(procArr) != 2 {
		// Too many args
		slog.Info("GetInterval: Bad interval, incorrect args", "interval", intervalStr)
		return interval
	}

	// Success
	interval[0] = procArr[0]
	interval[1] = procArr[1]

	if interval[1] == "" {
		interval[1] = skiplist.STRINGMAX
	}

	slog.Info("GetInterval: Good interval", "arg[0]", interval[0], "arg[1]", interval[1])
	return interval
}

// implement subscribable interface
// subscribe to the collection
func (c *Collection) Subscribe(w http.ResponseWriter, r *http.Request, intervalStart, intervalEnd string) error {
	// create a new subscriber
	subscriber, err := subscribe.NewSubscriber(w, r, intervalStart, intervalEnd)
	if err != nil {
		return err
	}

	// add the subscriber to the subscriber manager
	c.subscriberManager.AddSubscriber(subscriber)

	//added new
	defer c.subscriberManager.RemoveSubscriber(subscriber)

	//go subscriber.Start()
	subscriber.Start()

	return nil
}

// notify subscribers of update messages
func (c *Collection) NotifySubscribersUpdate(msg []byte, intervalVal string) {
	c.subscriberManager.NotifySubscribersUpdate(msg, intervalVal)
}

// notify subscribers of delete messages
func (c *Collection) NotifySubscribersDelete(msg string, intervalVal string) {
	c.subscriberManager.NotifySubscribersDelete(msg, intervalVal)
}

// determine the interval
func determineInterval(doc interfaces.IDocument) string {
	docData, ok := doc.GetJSONDoc().(map[string]interface{})
	if !ok {
		return "general"
	}
	if category, exists := docData["category"].(string); exists {
		return category
	}
	return "general"
}

// create an update message
func createUpdateMessage(action string, doc interfaces.IDocument) ([]byte, error) {
	message := map[string]interface{}{
		"action":   action,
		"document": doc.GetRawDoc(),
	}

	// marshal the message
	msgBytes, err := json.Marshal(message)
	if err != nil {
		slog.Error("Failed to marshal update message", "error", err)
		return nil, err
	}
	return msgBytes, nil
}

// create a delete message
func createDeleteMessage(docPath string) (string, error) {
	message := map[string]interface{}{
		"action":   "delete",
		"document": docPath,
	}

	// marshal the message
	msgBytes, err := json.Marshal(message)
	if err != nil {
		slog.Error("Failed to marshal delete message", "error", err)
		return "", err
	}
	return string(msgBytes), nil
}

// determine the interval for deletion
func determineIntervalForDeletion(docPath string) string {
	parts := strings.Split(docPath, "/")
	for i, part := range parts {
		if part == "categories" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return "general"
}
