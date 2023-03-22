package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/benbjohnson/clock"
)

const (
	// How often to poll for new rows
	pollInterval = 2500 * time.Millisecond
	// Lease duration
	leaseDuration = 30 * time.Second
)

type Reminders struct {
	db        *sql.DB
	processor *Processor
}

func NewReminders(db *sql.DB) *Reminders {
	processor := NewProcessor(executeReminder, clock.New())
	return &Reminders{
		db:        db,
		processor: processor,
	}
}

// PollReminders periodically polls the database for the next reminder.
// This is a blocking function that should be called in a background goroutine.
func (r *Reminders) PollReminders(ctx context.Context) {
	t := time.NewTicker(pollInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			// Stop on context cancellation
			return

		case <-t.C:
			// Get the reminder reminder
			reminder, err := r.getNextReminder(ctx)
			if err != nil {
				log.Printf("Error retrieving reminder: %v", err)
				break
			}

			// No reminder, just continue
			if reminder == nil {
				break
			}

			// Add the reminder to the queue
			// TODO: Attempt to release the lease on the reminder
			err = r.processor.Enqueue(reminder)
			if err != nil {
				log.Printf("Error enqueueing reminder: %v", err)
				break
			}
			log.Printf("Enqueued reminder %s - scheduled for %s", reminder.Key(), reminder.ExecutionTime.Local().Format(time.RFC822))
		}
	}
}

func (r *Reminders) getNextReminder(ctx context.Context) (*Reminder, error) {
	now := time.Now().UnixMilli()

	// Select the next reminder that is scheduled to be executed within 5s from now and that does not have an active lease
	// The row is atomically updated to acquire a lease
	q := `UPDATE reminders
		SET lease_time = ?
		WHERE target IN (
			SELECT target
			FROM reminders
			WHERE 
				execution_time < ?
				AND lease_time < ?
			ORDER BY execution_time ASC
			LIMIT 1
		)
		RETURNING target, execution_time, period, ttl, data`
	dbRes, err := r.db.QueryContext(ctx, q, now, now+5000, now+leaseDuration.Milliseconds())
	if err != nil {
		// Ignore ErrNoRows
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	defer dbRes.Close()

	// If the result is empty, return
	if !dbRes.Next() {
		return nil, nil
	}

	// Scan the result
	var (
		res                        Reminder
		target                     string
		executionTime, period, ttl int64
	)
	err = dbRes.Scan(&target, &executionTime, &period, &ttl, &res.Data)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(target, "/")
	res.ActorType = parts[0]
	res.ActorID = parts[1]
	res.Name = parts[2]
	res.ExecutionTime = time.Unix(executionTime/1000, (executionTime%1000)*10e6)
	res.Period = time.Duration(period) * time.Millisecond
	res.TTL = time.Unix(ttl/1000, (ttl%1000)*10e6)

	return &res, nil
}
