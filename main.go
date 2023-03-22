package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	_ "modernc.org/sqlite"

	"reminders-demo/pkg/reminders"
)

func main() {
	// Connect to the database
	db, err := connectDB("data.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Ensure that the table exists
	err = ensureTable(db)
	if err != nil {
		log.Fatal(err)
	}

	// Create the reminders object
	reminders := NewReminders(db)

	// Poll for reminders
	go reminders.PollReminders(context.Background())

	// Start a server to get user input
	go reminders.startServer()

	// Sleep until context is canceled
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt, os.Kill)
	<-sigCh
	log.Println("Shutting down")
}

// Invoked when a reminder is executed.
func executeReminder(r *reminders.Reminder) {
	log.Printf("Executed reminder %s - scheduled for %s", r.Key(), r.ExecutionTime.Local().Format(time.RFC822))
}

func connectDB(file string) (*sql.DB, error) {
	connString := getConnectionString(file)
	return sql.Open("sqlite", connString)
}

func getConnectionString(file string) string {
	busyTimeoutMs := 2000
	qs := url.Values{
		"_txlock": []string{"immediate"},
		"_pragma": []string{
			"journal_mode(WAL)",
			fmt.Sprintf("busy_timeout(%d)", busyTimeoutMs),
		},
	}

	return "file:" + file + "?" + qs.Encode()
}

func ensureTable(db *sql.DB) error {
	_, err := db.ExecContext(context.TODO(),
		`CREATE TABLE IF NOT EXISTS reminders (
			target TEXT NOT NULL PRIMARY KEY,
			execution_time INTEGER NOT NULL,
			period INTEGER,
			ttl INTEGER,
			data BLOB,
			lease_time INTEGER NOT NULL
		) WITHOUT ROWID;

		CREATE INDEX IF NOT EXISTS execution_time_idx ON reminders (execution_time ASC);
		CREATE INDEX IF NOT EXISTS lease_time_idx ON reminders (lease_time ASC);
		`,
	)
	return err
}
