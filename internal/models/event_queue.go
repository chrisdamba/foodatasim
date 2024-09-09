package models

import (
	"container/heap"
	"sync"
	"time"
)

const (
	EventPlaceOrder            = "PlaceOrder"
	EventPrepareOrder          = "PrepareOrder"
	EventOrderReady            = "OrderReady"
	EventAssignDeliveryPartner = "AssignDeliveryPartner"
	EventPickUpOrder           = "PickUpOrder"
	EventOrderInTransit        = "OrderInTransit"
	EventCheckDeliveryStatus   = "CheckDeliveryStatus"

	EventDeliverOrder             = "DeliverOrder"
	EventCancelOrder              = "CancelOrder"
	EventUpdateRestaurantStatus   = "UpdateRestaurantStatus"
	EventUpdatePartnerLocation    = "UpdatePartnerLocation"
	EventMoveDeliveryPartner      = "MoveDeliveryPartner"
	EventDeliveryPartnerGoOffline = "DeliveryPartnerGoOffline"
	EventDeliveryPartnerGoOnline  = "DeliveryPartnerGoOnline"
	EventUserRateOrder            = "UserRateOrder"
	EventRestaurantOpenClose      = "RestaurantOpenClose"
	EventUpdateTraffic            = "UpdateTraffic"
	EventAddNewUser               = "AddNewUser"
	EventUpdateUserBehaviour      = "UpdateUserBehaviour"
	EventAddNewRestaurant         = "AddNewRestaurant"
	EventAddNewDeliveryPartner    = "AddNewDeliveryPartner"
	EventGenerateReview           = "GenerateReview"
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
	mutex  sync.Mutex
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
func (eq *EventQueue) Enqueue(event *Event) {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()
	heap.Push((*eventHeap)(&eq.events), event)
}

// Dequeue removes and returns the earliest event from the queue
func (eq *EventQueue) Dequeue() *Event {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()
	if len(eq.events) == 0 {
		return nil
	}
	return heap.Pop((*eventHeap)(&eq.events)).(*Event)
}

// Peek returns the earliest event without removing it
func (eq *EventQueue) Peek() *Event {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()
	if len(eq.events) == 0 {
		return nil
	}
	return eq.events[0]
}

// IsEmpty returns true if the queue is empty
func (eq *EventQueue) IsEmpty() bool {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()
	return len(eq.events) == 0
}

// Len returns the number of events in the queue
func (eq *EventQueue) Len() int {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()
	return len(eq.events)
}

func (eq *EventQueue) DequeueBatch(maxBatchSize int) []*Event {
	eq.mutex.Lock()
	defer eq.mutex.Unlock()

	batchSize := min(maxBatchSize, len(eq.events))
	batch := make([]*Event, 0, batchSize)

	for i := 0; i < batchSize; i++ {
		event := heap.Pop((*eventHeap)(&eq.events)).(*Event)
		batch = append(batch, event)
	}

	return batch
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
