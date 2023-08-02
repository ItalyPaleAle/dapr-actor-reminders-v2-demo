package reminders

import (
	"encoding/json"
	"time"
)

type Reminder struct {
	ActorID       string          `json:"actorID,omitempty"`
	ActorType     string          `json:"actorType,omitempty"`
	Name          string          `json:"name,omitempty"`
	ExecutionTime time.Time       `json:"executionTime,omitempty"`
	Period        time.Duration   `json:"period,omitempty"`
	TTL           time.Time       `json:"expirationTime,omitempty"`
	Data          json.RawMessage `json:"data,omitempty"`

	// Lease time is used internally to make sure the reminder hasn't been modified while it's being executed
	LeaseTime int64 `json:"-"`
}

// Key returns the key for this unique reminder.
func (r Reminder) Key() string {
	return r.ActorType + "/" + r.ActorID + "/" + r.Name
}

// ScheduledTime returns the time the reminder is scheduled to be executed at.
// This is implemented to comply with the queueable interface.
func (r Reminder) ScheduledTime() time.Time {
	return r.ExecutionTime
}
