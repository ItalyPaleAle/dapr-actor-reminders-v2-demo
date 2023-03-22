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

// NextTick returns the time the reminder should tick again next.
func (r Reminder) NextTick() time.Time {
	return r.ExecutionTime
}
