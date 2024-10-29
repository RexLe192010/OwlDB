// This package is responsible for handling the subscribers and sending updates to them.
package subscribe

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type Subscriber struct {
	ID            string              //The identifier for the sub
	Updates       chan []byte         //Channel for updates sent to sub
	IntervalStart string              //Interval start
	IntervalEnd   string              //Interval end
	Conn          http.ResponseWriter //http response writer
	Flusher       http.Flusher        //For streaming
	CloseNotify   <-chan struct{}     //To know when client closed
	Closed        bool                //Tells us if its closed or not
	mu            sync.Mutex          //To protect access to above
}

type SubscriberManager struct {
	subscribers map[string]*Subscriber // Map of subscribers
	mu          sync.RWMutex           // To protect access to above
}

// Create a new subscriber
func NewSubscriber(w http.ResponseWriter, r *http.Request, intervalStart, intervalEnd string) (*Subscriber, error) {
	slog.Info("subscribe NewSubscriber: Creating new subscriber") // Logging that a new subscriber is being created
	flusher, ok := w.(http.Flusher)
	if !ok {
		slog.Error("subscribe NewSubscriber: Streaming unsupported") // Logging that streaming is unsupported
		return nil, fmt.Errorf("streaming unsupported")
	}

	// Set the headers related to server-sent events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	s := &Subscriber{
		ID:            generateUniqueID(),
		Updates:       make(chan []byte, 100),
		IntervalStart: intervalStart,
		IntervalEnd:   intervalEnd,
		Conn:          w,
		Flusher:       flusher,
		CloseNotify:   r.Context().Done(),
		Closed:        false,
	}

	return s, nil
}

// Start the subscriber
func (s *Subscriber) Start() {
	slog.Info("subscribe Start: Subscriber started", "ID", s.ID) // Logging that subscriber started
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.CloseNotify:
			// The client has closed the connection
			slog.Info("subscribe Start: Subscriber connection closed by client", "ID", s.ID)
			s.Close()
			return
		case update, ok := <-s.Updates:
			// We got an update from the server
			if !ok {
				// The channel has been closed
				slog.Info("subscribe Start: Subscriber updates channel closed", "ID", s.ID)
				s.Close()
				return
			}
			s.send(update)
		case <-ticker.C:
			// Send a comment to keep the connection alive
			slog.Info("subscribe Start: Sending keep-alive comment to subscriber", "ID", s.ID)
			s.sendComment(": keep-alive")
		}
	}
}

// This is a helper function to send comments to the subscriber's connection, to keep the connection alive
func (s *Subscriber) sendComment(comment string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Closed {
		// If the subscriber is closed, we don't want to send comments to it
		return
	}

	_, err := fmt.Fprintf(s.Conn, "%s\n\n", comment)
	if err != nil {
		// If there was an error sending the comment, log the error and close the subscriber
		slog.Error("subscribe sendComment: Error sending comment to subscriber", "ID", s.ID, "error", err)
		s.Close()
		return
	}
	s.Flusher.Flush()
}

// func (s *Subscriber) writeComment(comment string) {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()

// 	if s.Closed {
// 		return
// 	}

// 	_, err := fmt.Fprintf(s.Conn, "%s\n\n", comment)
// 	if err != nil {
// 		slog.Error("subscribe WriteComment: Error sending comment to subscriber", "ID", s.ID, "error", err)
// 		s.Close()
// 		return
// 	}
// 	s.Flusher.Flush()
// }

// Writing updates to the subscriber's connection
func (s *Subscriber) send(update []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Closed {
		// If the subscriber is closed, we don't want to send updates to it
		return
	}

	eventID := time.Now().UnixNano() / int64(time.Millisecond)
	eventType := determineEventType(update)

	_, err := fmt.Fprintf(s.Conn, "id: %d\n", eventID)
	if err != nil {
		// If there was an error sending the event ID, log the error and close the subscriber
		slog.Error("Error sending event ID to subscriber", "ID", s.ID, "error", err)
		s.Close()
		return
	}

	_, err = fmt.Fprintf(s.Conn, "event: %s\n", eventType)
	if err != nil {
		// If there was an error sending the event type, log the error and close the subscriber
		slog.Error("Error sending event type to subscriber", "ID", s.ID, "error", err)
		s.Close()
		return
	}

	_, err = fmt.Fprintf(s.Conn, "data: %s\n\n", update)
	if err != nil {
		// If there was an error sending the data, log the error and close the subscriber
		slog.Error("Error sending data to subscriber", "ID", s.ID, "error", err)
		s.Close()
		return
	}

	s.Flusher.Flush()
	slog.Debug("Event sent successfully", "SubscriberID", s.ID)

}

// To determine the event type of the update
// event type includes update and delete
func determineEventType(update []byte) string {
	var msg map[string]interface{}
	err := json.Unmarshal(update, &msg)
	if err != nil {
		return "message"
	}
	if action, ok := msg["action"].(string); ok {
		return action
	}
	return "message"
}

// Close the subscriber
func (s *Subscriber) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Closed {
		return
	}

	s.Closed = true
	close(s.Updates)
	slog.Info("subscribe Close: Subscriber closed", "ID", s.ID)
}

// Send an update to the subscriber
func (s *Subscriber) SendUpdate(msg []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Closed {
		// If the subscriber is closed, we don't want to send updates to it
		return
	}

	select {
	case s.Updates <- msg:
	case <-time.After(5 * time.Second):
		// 5 seconds timeout for sending update
		slog.Info("subscribe SendUpdate: Subscriber send update timeout", "ID", s.ID)
		s.Close()
	}
}

// Generate a unique ID for the subscriber, which is a timestamp
func generateUniqueID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Create a new subscriber manager
func NewSubscriberManager() *SubscriberManager {
	return &SubscriberManager{
		subscribers: make(map[string]*Subscriber),
	}
}

// Add a subscriber to the manager
func (m *SubscriberManager) AddSubscriber(s *Subscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribers[s.ID] = s
	slog.Info("subscribe AddSubscriber: Subscriber added", "ID", s.ID)
}

// Remove a subscriber from the manager
func (m *SubscriberManager) RemoveSubscriber(s *Subscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.subscribers, s.ID)
	slog.Info("subscribe RemoveSubscriber: Subscriber removed", "ID", s.ID)
}

// Notify subscribers of update messages
func (m *SubscriberManager) NotifySubscribersUpdate(msg []byte, intervalComp string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.subscribers {
		// check if the subscriber's interval is within the interval of the message
		if s.IntervalStart <= intervalComp && s.IntervalEnd >= intervalComp {
			slog.Info("Notifying subscriber update", "SubscriberID", s.ID, "IntervalComp", intervalComp)
			s.SendUpdate(msg)
		}
	}
}

// Notify subscribers of delete messages
func (m *SubscriberManager) NotifySubscribersDelete(msg string, intervalComp string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.subscribers {
		// check if the subscriber's interval is within the interval of the message
		if s.IntervalStart <= intervalComp && s.IntervalEnd >= intervalComp {
			slog.Info("Notifying subscriber deletion", "SubscriberID", s.ID, "IntervalComp", intervalComp)
			s.SendUpdate([]byte(msg))
		}
	}
}

// Cleanup the subscribers
func (m *SubscriberManager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.subscribers {
		if s.Closed {
			// If the subscriber is closed, remove it from the manager
			delete(m.subscribers, id)
			slog.Info("Clean now", "ID", id)
		}
	}
}
