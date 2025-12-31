// Package transport provides abstractions for communication between services.
package transport

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
)

// Common errors returned by MessageBus implementations
var (
	// ErrMessageBusNotAvailable is returned when a message bus operation is
	// requested but no message bus is available
	ErrMessageBusNotAvailable = errors.New("message bus not available")

	// ErrMessageBusClosed is returned when operations are attempted on a closed message bus
	ErrMessageBusClosed = errors.New("message bus closed")

	// ErrNilCallback is returned when a nil callback is provided to Subscribe
	ErrNilCallback = errors.New("callback cannot be nil")
)

// MessageBus is the interface for distributing messages in clustered environments.
// It provides a publish-subscribe mechanism where publishers can send messages
// to named channels, and subscribers can register callbacks to receive those messages.
//
// Thread Safety:
// All implementations of MessageBus must be safe for concurrent use from multiple goroutines.
// Callbacks are executed asynchronously to prevent publishers from being blocked.
//
// Usage after Close:
// After Close() is called, all other operations should return an error indicating
// the bus is closed. It is the caller's responsibility to handle these errors appropriately.
type MessageBus interface {
	// Publish sends a message to all subscribers for a channel.
	// The provided context can be used to cancel the operation.
	// Returns an error if the bus is closed or if the context is canceled.
	Publish(ctx context.Context, channel string, message []byte) error

	// Subscribe registers a callback to be invoked when messages arrive on a channel.
	// Returns a subscription ID that can be used to unsubscribe.
	// The context can be used to cancel the operation, but not the subscription itself.
	// Returns an error if the bus is closed, the callback is nil, or if the context is canceled.
	Subscribe(ctx context.Context, channel string, callback func(message []byte)) (string, error)

	// Unsubscribe removes a subscription by ID.
	// The context can be used to cancel the operation.
	// Returns an error if the bus is closed or if the context is canceled.
	// It is not an error to unsubscribe an ID that doesn't exist.
	Unsubscribe(ctx context.Context, subscriptionID string) error

	// Close shuts down the message bus.
	// After Close is called, all other operations will return ErrMessageBusClosed.
	// It is safe to call Close multiple times.
	Close(ctx context.Context) error
}

// InMemoryMessageBus provides a simple in-memory implementation of MessageBus
// Safe for concurrent use from multiple goroutines
type InMemoryMessageBus struct {
	subscribers map[string]map[string]func(message []byte)
	mu          sync.RWMutex
	closed      bool
}

var _ MessageBus = &InMemoryMessageBus{}

// NewInMemoryMessageBus creates a new in-memory message bus
func NewInMemoryMessageBus() *InMemoryMessageBus {
	return &InMemoryMessageBus{
		subscribers: make(map[string]map[string]func(message []byte)),
	}
}

// Subscribe registers a callback for the given channel
func (b *InMemoryMessageBus) Subscribe(ctx context.Context, channel string, callback func(message []byte)) (string, error) {
	if callback == nil {
		return "", ErrNilCallback
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return "", ErrMessageBusClosed
	}

	// Initialize channel map if needed
	if _, ok := b.subscribers[channel]; !ok {
		b.subscribers[channel] = make(map[string]func(message []byte))
	}

	// Generate simple subscription ID
	subID := generateSubscriptionID()

	// Store callback
	b.subscribers[channel][subID] = callback

	return subID, nil
}

// generateSubscriptionID creates a unique subscription ID using UUID
func generateSubscriptionID() string {
	return "sub-" + uuid.New().String()
}

// Publish sends a message to all subscribers of a channel
func (b *InMemoryMessageBus) Publish(ctx context.Context, channel string, message []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrMessageBusClosed
	}

	channelSubs, ok := b.subscribers[channel]
	if !ok || len(channelSubs) == 0 {
		b.mu.RUnlock()
		return nil
	}

	// Copy the message to prevent data races
	msgCopy := make([]byte, len(message))
	copy(msgCopy, message)

	// Copy the subscribers to avoid holding the lock during callbacks
	callbacks := make([]func([]byte), 0, len(channelSubs))
	for _, cb := range channelSubs {
		callbacks = append(callbacks, cb)
	}
	b.mu.RUnlock()

	// Deliver to each subscriber
	for _, cb := range callbacks {
		c := cb // Capture for goroutine
		go func() {
			defer func() {
				// Recover from panics in callbacks
				_ = recover()
			}()
			c(msgCopy)
		}()
	}

	return nil
}

// Unsubscribe removes a subscription by ID
func (b *InMemoryMessageBus) Unsubscribe(ctx context.Context, subscriptionID string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ErrMessageBusClosed
	}

	// Find the subscription in all channels
	for channel, subs := range b.subscribers {
		if _, ok := subs[subscriptionID]; ok {
			delete(subs, subscriptionID)

			// If channel has no more subscriptions, clean it up
			if len(subs) == 0 {
				delete(b.subscribers, channel)
			}

			return nil
		}
	}

	// Not finding the subscription ID is not an error
	return nil
}

// Close shuts down the message bus
func (b *InMemoryMessageBus) Close(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.subscribers = nil
	b.closed = true
	return nil
}
