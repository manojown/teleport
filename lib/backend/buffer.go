/*
Copyright 2018-2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backend

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// CircularBuffer implements in-memory circular buffer
// of predefined size, that is capable of fan-out of the backend events.
type CircularBuffer struct {
	sync.Mutex
	*log.Entry
	ctx      context.Context
	cancel   context.CancelFunc
	events   []Event
	start    int
	end      int
	size     int
	watchers []*BufferWatcher
}

// NewCircularBuffer returns a new instance of circular buffer
func NewCircularBuffer(ctx context.Context, size int) (*CircularBuffer, error) {
	if size <= 0 {
		return nil, trace.BadParameter("circular buffer size should be > 0")
	}
	ctx, cancel := context.WithCancel(ctx)
	buf := &CircularBuffer{
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentBuffer,
		}),
		ctx:    ctx,
		cancel: cancel,
		events: make([]Event, size),
		start:  -1,
		end:    -1,
		size:   0,
	}
	return buf, nil
}

// Reset resets all events from the queue
// and closes all active watchers
func (c *CircularBuffer) Reset() {
	c.Lock()
	defer c.Unlock()
	for _, w := range c.watchers {
		w.Close()
	}
	c.watchers = nil
	c.start = -1
	c.end = -1
	c.size = 0
	for i := 0; i < len(c.events); i++ {
		c.events[i] = Event{}
	}
}

// Close closes circular buffer and all watchers
func (c *CircularBuffer) Close() error {
	c.cancel()
	c.Reset()
	return nil
}

// Size returns circular buffer size
func (c *CircularBuffer) Size() int {
	return c.size
}

// Events returns a copy of records as arranged from start to end
func (c *CircularBuffer) Events() []Event {
	c.Lock()
	defer c.Unlock()
	return c.eventsCopy()
}

// eventsCopy returns a copy of events as arranged from start to end
func (c *CircularBuffer) eventsCopy() []Event {
	if c.size == 0 {
		return nil
	}
	var out []Event
	for i := 0; i < c.size; i++ {
		index := (c.start + i) % len(c.events)
		if out == nil {
			out = make([]Event, 0, c.size)
		}
		out = append(out, c.events[index])
	}
	return out
}

// PushBatch pushes elements to the queue as a batch
func (c *CircularBuffer) PushBatch(events []Event) {
	c.Lock()
	defer c.Unlock()

	for i := range events {
		c.push(events[i])
	}
}

// Push pushes elements to the queue
func (c *CircularBuffer) Push(r Event) {
	c.Lock()
	defer c.Unlock()
	c.push(r)
}

func (c *CircularBuffer) push(r Event) {
	if c.size == 0 {
		c.start = 0
		c.end = 0
		c.size = 1
	} else if c.size < len(c.events) {
		c.end = (c.end + 1) % len(c.events)
		c.events[c.end] = r
		c.size++
	} else {
		c.end = c.start
		c.start = (c.start + 1) % len(c.events)
	}
	c.events[c.end] = r
	c.fanOutEvent(r)
}

func matchPrefix(prefixes [][]byte, e Event) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, prefix := range prefixes {
		if bytes.HasPrefix(e.Item.Key, prefix) {
			return true
		}
	}
	return false
}

func (c *CircularBuffer) fanOutEvent(r Event) {
	for i, watcher := range c.watchers {
		if !matchPrefix(watcher.Prefixes, r) {
			continue
		}
		select {
		case watcher.eventsC <- r:
		case <-c.ctx.Done():
			return
		default:
			c.Warningf("Closing %v, buffer overflow at %v elements.", watcher, len(watcher.eventsC))
			watcher.Close()
			c.watchers = append(c.watchers[:i], c.watchers[i+1:]...)
		}
	}
}

// NewWatcher adds a new watcher to the events buffer
func (c *CircularBuffer) NewWatcher(ctx context.Context, watch Watch) (Watcher, error) {
	c.Lock()
	defer c.Unlock()

	select {
	case <-c.ctx.Done():
		return nil, trace.BadParameter("buffer is closed")
	default:
	}

	if watch.QueueSize == 0 {
		watch.QueueSize = len(c.events)
	}

	closeCtx, cancel := context.WithCancel(ctx)
	w := &BufferWatcher{
		Watch:    watch,
		eventsC:  make(chan Event, watch.QueueSize),
		ctx:      closeCtx,
		cancel:   cancel,
		capacity: watch.QueueSize,
	}
	c.Debugf("Add %v.", w)
	select {
	case w.eventsC <- Event{Type: OpInit}:
	case <-c.ctx.Done():
		return nil, trace.BadParameter("buffer is closed")
	default:
		c.Warningf("Closing %v, buffer overflow.", w)
		w.Close()
		return nil, trace.BadParameter("buffer overflow")
	}
	c.watchers = append(c.watchers, w)
	return w, nil
}

func max(a, b int) int {
	if a > b {
		return b
	}
	return a
}

// BufferWatcher is a watcher connected to the
// buffer and receiving fan-out events from the watcher
type BufferWatcher struct {
	Watch
	eventsC  chan Event
	ctx      context.Context
	cancel   context.CancelFunc
	capacity int
}

// String returns user-friendly representation
// of the buffer watcher
func (w *BufferWatcher) String() string {
	return fmt.Sprintf("Watcher(name=%v, prefixes=%v, capacity=%v, size=%v)", w.Name, string(bytes.Join(w.Prefixes, []byte(", "))), w.capacity, len(w.eventsC))
}

// Events returns events channel
func (w *BufferWatcher) Events() <-chan Event {
	return w.eventsC
}

// Done channel is closed when watcher is closed
func (w *BufferWatcher) Done() <-chan struct{} {
	return w.ctx.Done()
}

// Close closes the watcher, could
// be called multiple times
func (w *BufferWatcher) Close() error {
	w.cancel()
	return nil
}
