// Package document implements the document functionality as specified in the owlDB api.
// It contains a struct that represents a document and several methods including get, put, delete, and patch.
// It also contains a method for creating a new document.
package document

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/RICE-COMP318-FALL24/owldb-p1group70/collectionholder"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/errorMessage"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/interfaces"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/patcher"
	"github.com/RICE-COMP318-FALL24/owldb-p1group70/subscribe"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// A meta stores metadata about a document.
type meta struct {
	CreatedBy      string `json:"createdBy"`      // The user who created this JSON document.
	CreatedAt      int64  `json:"createdAt"`      // The time this JSON document was created.
	LastModifiedBy string `json:"lastModifiedBy"` // The last user who modified this JSON document.
	LastModifiedAt int64  `json:"lastModifiedAt"` // The last time that this JSON document was modified.
}

// A docoutput is a struct which represents the data to be output when a user requests a given document.
type docOutput struct {
	Path string      `json:"path"` // The relative path to this document.
	Doc  interface{} `json:"doc"`  // The actual JSON document represented by this object.
	Meta meta        `json:"meta"` // The metadata of this document.
}

// A document is a document plus a concurrent skiplist of collections, and a slice of subscribers.
type Document struct {
	output            docOutput                          // The document held in this object with extra meta data.
	children          *collectionholder.CollectionHolder // The set of collections this document holds.
	SubscriberManager *subscribe.SubscriberManager       // The subscribe manager of this document holds.
}

// Create a new document.
func New(path, user string, docBody interface{}) Document {
	newH := collectionholder.New()
	subscriberManager := subscribe.NewSubscriberManager()
	return Document{newOutput(path, user, docBody), &newH, subscriberManager} // make([]subscribe.Subscriber, 0)}
}

// Create a new docOutput.
func newOutput(path, user string, docBody interface{}) docOutput {
	return docOutput{path, docBody, newMeta(user)}
}

// Create a new metadata.
func newMeta(user string) meta {
	return meta{user, time.Now().UnixMilli(), user, time.Now().UnixMilli()}
}

// Handle a GET request on this document.
func (d *Document) GetDoc(w http.ResponseWriter, r *http.Request) {
	// handle subscribe mode request
	if r.URL.Query().Get("mode") == "subscribe" {
		// extract interval values from query
		intervalStart := r.URL.Query().Get("intervalStart")
		intervalEnd := r.URL.Query().Get("intervalEnd")

		err := d.Subscribe(w, r, intervalStart, intervalEnd)
		if err != nil {
			errorMessage.ErrorResponse(w, "Subscription failed: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// convert to JSON and write to response
	jsonDoc, err := d.GetJSONBody()
	if err != nil {
		errorMessage.ErrorResponse(w, "Error converting document to JSON", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonDoc)
	slog.Info("document GetDoc: success", "path", d.output.Path)
}

// Handle a PUT request with a path pointing to this document.
// Put a new collection in this document.
func (d *Document) PutColl(w http.ResponseWriter, r *http.Request, newName string, newColl interfaces.ICollection) {
	slog.Info("document: Putting collection", "name", newName)
	d.children.PutColl(w, r, newName, newColl)
}

// Handle a DELETE request with a path pointing to this document.
// Delete a collection from this document.
func (d *Document) DeleteColl(w http.ResponseWriter, r *http.Request, collName string) {
	slog.Info("document: Deleting collection", "name", collName)
	d.children.DeleteColl(w, r, collName)
}

// Find a collection associated with this document for other methods.
func (d *Document) GetColl(resource string) (interfaces.ICollection, bool) {
	slog.Info("document: Getting collection", "resource", resource)
	return d.children.GetColl(resource)
}

// Overwrite the body of a document upon recieving a put or patch.
func (d *Document) OverwriteBody(docBody interface{}, name string) {
	existingDocOutput := d.output
	existingDocOutput.Meta.LastModifiedAt = time.Now().UnixMilli()
	existingDocOutput.Meta.LastModifiedBy = name

	// Modify document contents
	existingDocOutput.Doc = docBody

	// Modify it again in the doc
	d.output = existingDocOutput

	// Wipes the children of this document
	newChildren := collectionholder.New()
	d.children = &newChildren
}

// Concatenate the path of this document with the input path.
func (d *Document) ConcatPath(path string) {
	// currently not in use, may need to change location of function
	d.output.Path += path
}

// apply a slice of  patches to the document
func (d *Document) ApplyPatches(patchData []patcher.Patch, schema *jsonschema.Schema) (patcher.PatchResponse, interface{}) {
	slog.Info("Applying patches to document", "path", d.output.Path)
	var result patcher.PatchResponse
	var err error

	// Apply each patch to the document
	newDoc := d.output.Doc
	for i, patch := range patchData {
		slog.Info("document ApplyPatches: Applying patch", "num", i, "patch", patch)
		newDoc, err = patcher.ApplyPatch(newDoc, patch)
		if err != nil {
			slog.Error("Error applying patch", "num", i, "error", err)
			result.Message = fmt.Sprintf("Error applying patch %d: %s", i, err.Error())
			result.PatchFailed = true
			return result, nil
		}
	}

	// Validate the document against the schema
	err = schema.Validate(newDoc)
	if err != nil {
		slog.Error("patched document does not conform to the schema", "error", err)
		result.Message = fmt.Sprintf("Patched document does not conform to the schema: %s", err.Error())
		result.PatchFailed = true
		return result, nil
	}

	// successfully applied patches
	result.Message = "Patches applied successfully"
	result.PatchFailed = false
	return result, newDoc
}

// Get the last modified from this document for conditional put.
func (d *Document) GetLastModified() int64 {
	return d.output.Meta.LastModifiedAt
}

// Get the original author of this document.
func (d *Document) GetOriginalAuthor() string {
	return d.output.Meta.CreatedBy
}

// Get the JSON Object that this document stores.
func (d *Document) GetJSONBody() ([]byte, error) {
	jsonBody, err := json.Marshal(d.output)
	if err != nil {
		// This should never happen
		slog.Error("Error marshalling doc body", "error", err)
		return nil, err
	}

	return jsonBody, err
}

// Get the JSON Object that this document stores.
func (d *Document) GetRawDoc() interface{} {
	return d.output
}

// Get the JSON Document that this document stores.
func (d *Document) GetJSONDoc() interface{} {
	return d.output.Doc
}

// implement subscribable interface
// Subscribe to this document.
func (d *Document) Subscribe(w http.ResponseWriter, r *http.Request, intervalStart, intervalEnd string) error {
	// Create a new subscriber
	subscriber, err := subscribe.NewSubscriber(w, r, intervalStart, intervalEnd)
	if err != nil {
		return err
	}

	d.SubscriberManager.AddSubscriber(subscriber) // Add the subscriber to the manager

	defer d.SubscriberManager.RemoveSubscriber(subscriber)

	slog.Info("document Subscribe: Subscriber added", "ID", subscriber.ID)

	message := map[string]interface{}{
		// the messsage to send to the subscriber
		"action":  "update",
		"content": d.GetRawDoc(),
	}

	update, err := json.Marshal(message)
	if err != nil {
		slog.Error("document Subscribe: Error marshalling update message", "error", err)
		return err
	}

	subscriber.SendUpdate(update)

	// Start the subscriber
	subscriber.Start()

	return nil
}

// Notify subscribers of update messages.
func (d *Document) NotifySubscribersUpdate(msg []byte, intervalVal string) {
	d.SubscriberManager.NotifySubscribersUpdate(msg, intervalVal)
}

// Notify subscribers of delete messages.
func (d *Document) NotifySubscribersDelete(msg string, intervalVal string) {
	d.SubscriberManager.NotifySubscribersDelete(msg, intervalVal)
}
