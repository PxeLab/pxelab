package eventbus

import (
	"sync"
	"sync/atomic"
)

type Event struct {
	Topic   string
	Payload any
}

type Handler func(Event)

type Bus struct {
	mu          sync.RWMutex
	subscribers map[string]map[int64]Handler
	counter     int64
}

func New() *Bus {
	return &Bus{
		subscribers: make(map[string]map[int64]Handler),
	}
}

func (b *Bus) Subscribe(topic string, handler Handler) int64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := atomic.AddInt64(&b.counter, 1)
	if _, ok := b.subscribers[topic]; !ok {
		b.subscribers[topic] = make(map[int64]Handler)
	}
	b.subscribers[topic][id] = handler
	return id
}

func (b *Bus) Unsubscribe(topic string, id int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if handlers, ok := b.subscribers[topic]; ok {
		delete(handlers, id)
	}
}

func (b *Bus) Publish(topic string, payload any) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if handlers, ok := b.subscribers[topic]; ok {
		for _, handler := range handlers {
			handler(Event{Topic: topic, Payload: payload})
		}
	}
}

// PublishAsync 异步非阻塞发布，日志等高频场景使用
func (b *Bus) PublishAsync(topic string, payload any) {
	go b.Publish(topic, payload)
}

func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers = make(map[string]map[int64]Handler)
}
