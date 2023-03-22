package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/benbjohnson/clock"

	"reminders-demo/pkg/reminders"
)

const (
	// How often to poll for new rows
	pollInterval = 2500 * time.Millisecond
	// When fetching reminders, only look for those scheduled to be executed within this time interval
	fetchAhead = 5 * time.Second
	// Lease duration
	leaseDuration = 30 * time.Second
)

type Reminders struct {
	db        *sql.DB
	processor *reminders.Processor
}

func NewReminders(db *sql.DB) *Reminders {
	r := &Reminders{
		db: db,
	}
	r.processor = reminders.NewProcessor(r.executeReminder, clock.New())
	return r
}

// AddReminder adds a reminder to be executed.
func (r *Reminders) AddReminder(ctx context.Context, reminder *reminders.Reminder) error {
	q := `INSERT OR REPLACE INTO reminders
			(target, execution_time, period, ttl, data, lease_time)
		VALUES (?, ?, ?, ?, ?, 0)`
	_, err := r.db.ExecContext(ctx, q,
		reminder.Key(),
		reminder.ExecutionTime.UnixMilli(),
		reminder.Period.Milliseconds(),
		reminder.TTL.UnixMilli(),
		reminder.Data,
	)

	// Remove the reminder from the processor in case was an existing one that was replaced and it's currently in our queue
	err = r.processor.Dequeue(reminder)
	if err != nil {
		return err
	}

	return err
}

// DeleteReminder removes a reminder.
func (r *Reminders) DeleteReminder(ctx context.Context, reminder *reminders.Reminder) error {
	// Delete from the database
	q := `DELETE FROM reminders WHERE target = ?`
	res, err := r.db.ExecContext(ctx, q, reminder.Key())
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		log.Printf("Reminder %s was not deleted", reminder.Key())
	}

	// Remove the reminder from the processor in case it is in our queue
	err = r.processor.Dequeue(reminder)
	if err != nil {
		return err
	}

	return nil
}

func (r *Reminders) executeReminder(reminder *reminders.Reminder) {
	err := r.doExecuteReminder(reminder)
	if err != nil {
		log.Printf("Error while attempting to execute reminder: %v", err)
	}
}

func (r *Reminders) doExecuteReminder(reminder *reminders.Reminder) error {
	// Delete the row from the database but only if it hasn't been modified yet
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	// Automatically rollback
	defer tx.Rollback()

	// TODO: If the reminder repeats, rather than deleting it, update its execution_time
	q := `DELETE FROM reminders
		WHERE target = ?
			AND lease_time = ?`
	res, err := tx.ExecContext(context.TODO(), q, reminder.Key(), reminder.LeaseTime)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to count affected rows: %w", err)
	}

	// If no rows were affected, it means that the reminder was either deleted by another process, or we somehow lost the lease
	// In either case, do not execute it
	if n == 0 {
		log.Printf("Reminder %s cannot be executed because we lost the lease or the reminder was deleted", reminder.Key())
		return nil
	}

	// Execute the reminder
	executeReminder(reminder)

	// Now commit the transaction
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
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
			err = r.processor.Enqueue(reminder)
			if err != nil {
				// TODO: Attempt to release the lease on the reminder
				log.Printf("Error enqueueing reminder: %v", err)
				break
			}
			log.Printf("Enqueued reminder %s - scheduled for %s", reminder.Key(), reminder.ExecutionTime.Local().Format(time.RFC822))
		}
	}
}

func (r *Reminders) getNextReminder(ctx context.Context) (*reminders.Reminder, error) {
	now := time.Now().UnixMilli()

	// Select the next reminder that is scheduled to be executed within 5s from now and that does not have an active lease
	// The row is atomically updated to acquire a lease
	q := `UPDATE reminders
		SET lease_time = ?
		WHERE ROWID IN (
			SELECT ROWID
			FROM reminders
			WHERE 
				execution_time < ?
				AND lease_time < ?
			ORDER BY execution_time ASC
			LIMIT 1
		)
		RETURNING target, execution_time, period, ttl, lease_time`
	dbRes, err := r.db.QueryContext(ctx, q,
		now, now+fetchAhead.Milliseconds(), now-leaseDuration.Milliseconds(),
	)
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
		res                        reminders.Reminder
		target                     string
		executionTime, period, ttl int64
	)
	err = dbRes.Scan(&target, &executionTime, &period, &ttl, &res.LeaseTime)
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
