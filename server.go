package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"reminders-demo/pkg/reminders"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

// Used in the demo app to have a way to pass input to the server
func (rm *Reminders) startServer() {
	port := os.Getenv("PORT")
	if port == "" || port == "0" {
		port = "3000"
	}

	// Create the router
	router := chi.NewRouter()
	router.Use(middleware.Logger)

	// POST /reminder - Create or update a reminder
	router.Post("/reminder", func(w http.ResponseWriter, r *http.Request) {
		req := &struct {
			ActorID       string `json:"actorID,omitempty"`
			ActorType     string `json:"actorType,omitempty"`
			Name          string `json:"name,omitempty"`
			ExecutionTime string `json:"executionTime,omitempty"`
		}{}
		err := json.NewDecoder(r.Body).Decode(req)
		if err != nil {
			w.Write([]byte("Error parsing request body: " + err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if req.ActorType == "" {
			w.Write([]byte("actorType is empty"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.ActorID == "" {
			w.Write([]byte("actorId is empty"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			w.Write([]byte("name is empty"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.ExecutionTime == "" {
			w.Write([]byte("executionTime is empty"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// If executionTime is in the format "+duration", interpret it as time from now
		if len(req.ExecutionTime) > 1 && req.ExecutionTime[0] == '+' {
			dur, err := time.ParseDuration(req.ExecutionTime[1:])
			if err != nil {
				w.Write([]byte("Failed to parse executionTime as relative time: " + err.Error()))
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			req.ExecutionTime = time.Now().Add(dur).Format(time.RFC3339)
		}

		// Create the Reminder object
		executionTime, err := time.Parse(time.RFC3339, req.ExecutionTime)
		if err != nil {
			w.Write([]byte("Failed to parse executionTime: " + err.Error()))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		reminder := &reminders.Reminder{
			ActorType:     req.ActorType,
			ActorID:       req.ActorID,
			Name:          req.Name,
			ExecutionTime: executionTime,
		}
		err = rm.AddReminder(r.Context(), reminder)
		if err != nil {
			w.Write([]byte("Failed to add reminder: " + err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	// DELETE /reminder - Deletes a reminder
	router.Delete("/reminder", func(w http.ResponseWriter, r *http.Request) {
		// Use Reminder to get actorType, actorID, name
		reminder := &reminders.Reminder{}
		err := json.NewDecoder(r.Body).Decode(reminder)
		if err != nil {
			w.Write([]byte("Error parsing request body: " + err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = rm.DeleteReminder(r.Context(), reminder)
		if err != nil {
			w.Write([]byte("Failed to delete reminder: " + err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	// Start the server
	log.Printf("Server listening on http://127.0.0.1:%s", port)
	err := http.ListenAndServe("127.0.0.1:"+port, router)
	if err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
