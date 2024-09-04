package models

import (
	"container/heap"
	"time"
)

// Event represents a simulation event
type Event struct {
	Time time.Time
	Type string
	Data interface{}
}

// EventQueue is a priority queue of events
type EventQueue struct {
	events []*Event
}

// eventHeap implements heap.Interface and holds Events
type eventHeap []*Event

func (h eventHeap) Len() int           { return len(h) }
func (h eventHeap) Less(i, j int) bool { return h[i].Time.Before(h[j].Time) }
func (h eventHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *eventHeap) Push(x interface{}) {
	*h = append(*h, x.(*Event))
}

func (h *eventHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// NewEventQueue creates a new EventQueue
func NewEventQueue() *EventQueue {
	return &EventQueue{events: make([]*Event, 0)}
}

// Enqueue adds an event to the queue
func (eq *EventQueue) Enqueue(event Event) {
	heap.Push((*eventHeap)(&eq.events), &event)
}

// Dequeue removes and returns the earliest event from the queue
func (eq *EventQueue) Dequeue() (Event, bool) {
	if len(eq.events) == 0 {
		return Event{}, false
	}
	event := heap.Pop((*eventHeap)(&eq.events)).(*Event)
	return *event, true
}

// Peek returns the earliest event without removing it
func (eq *EventQueue) Peek() (Event, bool) {
	if len(eq.events) == 0 {
		return Event{}, false
	}
	return *eq.events[0], true
}

// IsEmpty returns true if the queue is empty
func (eq *EventQueue) IsEmpty() bool {
	return len(eq.events) == 0
}

// Len returns the number of events in the queue
func (eq *EventQueue) Len() int {
	return len(eq.events)
}
