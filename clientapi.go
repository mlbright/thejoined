package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// runView is the JSON form of a run in list/detail responses.
type runView struct {
	ID      string          `json:"id"`
	State   RunState        `json:"state"`
	Target  string          `json:"target"`
	Started time.Time       `json:"started"`
	Metrics MetricsSnapshot `json:"metrics"`
}

func viewOf(r *Run) runView {
	return runView{
		ID:      r.ID,
		State:   r.State(),
		Target:  r.Spec.Target,
		Started: r.Started,
		Metrics: r.Snapshot(),
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// clientMux builds the REST control API routed to the given manager. Uses Go
// 1.22+ method+wildcard pattern routing from net/http.
func clientMux(m *Manager) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /runs", func(w http.ResponseWriter, r *http.Request) {
		var spec RunSpec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
			return
		}
		run, err := m.Start(spec)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, viewOf(run))
	})

	mux.HandleFunc("GET /runs", func(w http.ResponseWriter, r *http.Request) {
		runs := m.List()
		views := make([]runView, len(runs))
		for i, run := range runs {
			views[i] = viewOf(run)
		}
		writeJSON(w, http.StatusOK, views)
	})

	mux.HandleFunc("GET /runs/{id}", func(w http.ResponseWriter, r *http.Request) {
		run, ok := m.Get(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such run"})
			return
		}
		writeJSON(w, http.StatusOK, viewOf(run))
	})

	mux.HandleFunc("POST /runs/{id}/stop", func(w http.ResponseWriter, r *http.Request) {
		if !m.Stop(r.PathValue("id")) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such run"})
			return
		}
		run, _ := m.Get(r.PathValue("id"))
		writeJSON(w, http.StatusOK, viewOf(run))
	})

	return mux
}

const maxKeptRuns = 100

// runClient starts the client-mode control API on the given port and blocks.
func runClient(port string) error {
	m := newManager(context.Background(), maxKeptRuns)
	log.Printf("client mode listening on :%s (control API)", port)
	return http.ListenAndServe(":"+port, clientMux(m))
}
