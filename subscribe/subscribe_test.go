package subscribe

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestNewSubscriber tests the NewSubscriber function
func TestNewSubscriber(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/subscribe", nil)

	intervalStart := "a"
	intervalEnd := "z"

	subscriber, err := NewSubscriber(rr, req, intervalStart, intervalEnd)
	if err != nil {
		t.Fatalf("Failed to create subscriber: %v", err)
	}

	if subscriber.IntervalStart != intervalStart || subscriber.IntervalEnd != intervalEnd {
		t.Errorf("Sub intervals not set correctly")
	}

	if subscriber.Closed {
		t.Errorf("Sub should not be closed upon creation")
	}
}

// TestSubscriberSendUpdate tests the SendUpdate function of a subscriber
func TestSubscriberSendUpdate(t *testing.T) {
	pr, pw := io.Pipe()
	flusher := &testFlusher{}

	rw := &responseWriterWithFlusher{
		ResponseWriter: httptest.NewRecorder(),
		writer:         pw,
		flusher:        flusher,
	}

	req := httptest.NewRequest("GET", "/subscribe", nil)

	subscriber, err := NewSubscriber(rw, req, "", "")
	if err != nil {
		t.Fatalf("Failed to create sub: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		subscriber.Start()
	}()

	updateMsg := []byte(`{"message":"test update"}`)
	subscriber.SendUpdate(updateMsg)

	buffer := make([]byte, 1024)
	n, err := pr.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read from sub output: %v", err)
	}

	output := string(buffer[:n])
	expectedOutput := "data: {\"message\":\"test update\"}\n\n"

	if output != expectedOutput {
		t.Errorf("Expected output %q, got %q", expectedOutput, output)
	}

	subscriber.Close()

	wg.Wait()
}

// TestSubscriberManager tests the construction and operation of a SubscriberManager
func TestSubscriberManager(t *testing.T) {
	manager := NewSubscriberManager()

	// construct two subscribers with different intervals
	sub1 := &Subscriber{
		ID:            "sub1",
		IntervalStart: "a",
		IntervalEnd:   "m",
		Updates:       make(chan []byte, 10),
		Closed:        false,
		mu:            sync.Mutex{},
	}

	sub2 := &Subscriber{
		ID:            "sub2",
		IntervalStart: "n",
		IntervalEnd:   "z",
		Updates:       make(chan []byte, 10),
		Closed:        false,
		mu:            sync.Mutex{},
	}

	// add the subscribers to the manager
	manager.AddSubscriber(sub1)
	manager.AddSubscriber(sub2)

	updateMsg := []byte(`{"message":"update in h"}`)
	manager.NotifySubscribersUpdate(updateMsg, "h")

	select {
	case msg := <-sub1.Updates:
		if !bytes.Equal(msg, updateMsg) {
			t.Errorf("sub1 got incorrect update: %s", msg)
		}
	case <-time.After(time.Second):
		t.Error("sub1 didnt receive update")
	}

	select {
	case msg := <-sub2.Updates:
		t.Errorf("sub2 got but shouldn't have gotten update: %s", msg)
	case <-time.After(100 * time.Millisecond):
	}

	manager.RemoveSubscriber(sub1)
	updateMsg2 := []byte(`{"message":"update in p"}`)
	manager.NotifySubscribersUpdate(updateMsg2, "p")

	select {
	case msg := <-sub2.Updates:
		if !bytes.Equal(msg, updateMsg2) {
			t.Errorf("sub2 received incorrect update: %s", msg)
		}
	case <-time.After(time.Second):
		t.Error("sub2 didn't receive update")
	}

	select {
	case msg := <-sub1.Updates:
		t.Errorf("sub1 should not have received update after removal, but got: %s", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

type responseWriterWithFlusher struct {
	http.ResponseWriter
	writer  io.Writer
	flusher http.Flusher
}

func (rw *responseWriterWithFlusher) Write(p []byte) (int, error) {
	return rw.writer.Write(p)
}

func (rw *responseWriterWithFlusher) Flush() {
	rw.flusher.Flush()
}

type testFlusher struct{}

func (f *testFlusher) Flush() {}
