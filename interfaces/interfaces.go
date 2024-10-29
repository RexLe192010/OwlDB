// Package interfaces contains interfaces of common data structures and methods used in the project.

package interfaces

import (
	"net/http"

	"github.com/RICE-COMP318-FALL24/owldb-p1group70/patcher"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Interface for a document.
type IDocument interface {
	// Get the docoutput resource (json + metadata)
	GetRawDoc() interface{}

	// Get the JSON document
	GetJSONDoc() interface{}

	// HTTP handler for GET requests
	GetDoc(w http.ResponseWriter, r *http.Request)
}

// Interface for a collection.
// A collection is a collection of documents.
type ICollection interface {
	// Get a resource from this collection
	FindDoc(resource string) (IDocument, bool)

	// HTTP handler for GET requests on collections (query collection)
	GetDoc(w http.ResponseWriter, r *http.Request)

	// HTTP handler for PUTs on document paths
	PutDoc(w http.ResponseWriter, r *http.Request, path string, newDoc IDocument)

	// HTTP handler for DELETEs on document paths
	DeleteDoc(w http.ResponseWriter, r *http.Request, docpath string)

	// HTTP handler for PATCH on document paths
	PatchDoc(w http.ResponseWriter, r *http.Request, docpath string, schema *jsonschema.Schema, name string)

	// HTTP handler for POST on document paths
	PostDoc(w http.ResponseWriter, r *http.Request, newDoc IDocument)

	//Subscription
	Subscribable
}

// Interface for a collection holder.
// A collection holder holds collections.
type ICollectionHolder interface {
	// HTTP handler for PUT requests on collection paths
	PutColl(w http.ResponseWriter, r *http.Request, dbpath string, newColl ICollection)

	// Get a resource from this object
	GetColl(resource string) (coll ICollection, found bool)

	// HTTP handler for DELETE requests on collections (manage collections)
	DeleteColl(w http.ResponseWriter, r *http.Request, collName string)
}

// An authenticator is something which can validate a login token
// as a valid user of a dbhandler or not.
type Authenticator interface {
	// ValidateToken tells if a token in a request is valid. Returns
	// true and the corresponding username if so, else writes an error to the input response writer.
	ValidateToken(w http.ResponseWriter, r *http.Request) (string, bool)
}

// A HasMetadata object allows storage and public retrieval of metadata
type HasMetadata interface {
	// Gets the original author of this document
	GetOriginalAuthor() string

	// Gets the last modified at field from
	// this document for conditional put.
	GetLastModified() int64
}

// A overwritable object allows being overwritten
type Overwriteable interface {
	// Overwrite the body of a document upon recieving a put or patch.
	OverwriteBody(docBody interface{}, name string)
}

// A postable object supports posting
type Postable interface {
	// Insert name to the end of the path string
	ConcatPath(path string)
}

// A Patchable object allows patching
type Patchable interface {
	ApplyPatch(patchData map[string]interface{}, schema *jsonschema.Schema, username string) error

	// Applys a slice of patches to this document.
	ApplyPatches(patches []patcher.Patch, schema *jsonschema.Schema) (patcher.PatchResponse, interface{})

	// Overwrite the body of a document upon recieving a put or patch.
	OverwriteBody(docBody interface{}, name string)
}

// A subscribable object allows the sending of messages to subscribers.
type Subscribable interface {
	Subscribe(w http.ResponseWriter, r *http.Request, intervalStart, intervalEnd string) error
	NotifySubscribersUpdate(msg []byte, intervalVal string)
	NotifySubscribersDelete(msg string, intervalVal string)
	// Notifies subscribers of update messages.
	// NotifySubscribersUpdate(msg []byte, intervalComp string)

	// Notifies subscribers of delete messages.
	// NotifySubscribersDelete(msg string, intervalComp string)
}
