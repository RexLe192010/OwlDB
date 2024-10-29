package structs

// A PutOutput stores the response to a put request.
type PutOutput struct {
	Uri string `json:"uri"` // The URI of the successful put operation.
}

// A CollSub is a wrapper for a subscriber to a collection.
type CollSub struct {
	// Subscriber    subscribe.Subscriber // The subscriber object.
	IntervalStart string // The start of the interval for this subscribers query.
	IntervalEnd   string // The end of the interval for this subscribers query.
}
