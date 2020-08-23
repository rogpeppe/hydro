// Package notifier implements a way for multiple watchers to get notified about a change
// to a changed value.
package notifier

import (
	"sync"
)

// Notifier represents a shared value that can be watched for changes. Methods on
// a Value may be called concurrently.
type Notifier struct {
	version int
	mu      sync.RWMutex
	wait    sync.Cond
	closed  bool
}

func (n *Notifier) needsInit() bool {
	return n.wait.L == nil
}

func (n *Notifier) init() {
	if n.needsInit() {
		n.wait.L = n.mu.RLocker()
	}
}

// Changed flags that the shared value has changed. All watchers will be notified.
func (n *Notifier) Changed() {
	n.mu.Lock()
	n.init()
	n.version++
	n.mu.Unlock()
	n.wait.Broadcast()
}

// Close closes the Value, unblocking any outstanding watchers.  Close always
// returns nil.
func (n *Notifier) Close() error {
	n.mu.Lock()
	n.init()
	n.closed = true
	n.mu.Unlock()
	n.wait.Broadcast()
	return nil
}

// Closed reports whether the value has been closed.
func (n *Notifier) Closed() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.closed
}

// Watch returns a Watcher that can be used to watch for changes to the value.
// If Changed hasn't been called on the Notifier, then the watcher
// will block until it is (or until it's closed).
func (n *Notifier) Watch() *Watcher {
	return &Watcher{notifier: n}
}

// Watcher represents a single watcher of a shared value.
type Watcher struct {
	notifier *Notifier
	version  int
	closed   bool
}

// Next blocks until there is a new value to be retrieved from the value that is
// being watched. It also unblocks when the value or the Watcher itself is
// closed. Next returns false if the value or the Watcher itself have been
// closed.
func (w *Watcher) Next() bool {
	n := w.notifier
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.needsInit() {
		n.mu.RUnlock()
		n.mu.Lock()
		n.init()
		n.mu.Unlock()
		n.mu.RLock()
	}

	// We can go around this loop a maximum of two times,
	// because the only thing that can cause a Wait to
	// return is for the condition to be triggered,
	// which can only happen if Changed is called (causing
	// the version to increment) or it is closed
	// causing the closed flag to be set.
	// Both these cases will cause Next to return.
	for {
		if w.version != n.version {
			w.version = n.version
			return true
		}
		if n.closed || w.closed {
			return false
		}

		// Wait releases the lock until triggered and then reacquires the lock,
		// thus avoiding a deadlock.
		n.wait.Wait()
	}
}

// Close closes the Watcher without closing the underlying
// value. It may be called concurrently with Next.
func (w *Watcher) Close() {
	w.notifier.mu.Lock()
	w.notifier.init()
	w.closed = true
	w.notifier.mu.Unlock()
	w.notifier.wait.Broadcast()
}
