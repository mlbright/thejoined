package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newAPI(t *testing.T) http.Handler {
	t.Helper()
	m := newManager(t.Context(), 16)
	return clientMux(m)
}

func TestAPICreateRunAndGet(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(handler))
	defer target.Close()
	api := httptest.NewServer(newAPI(t))
	defer api.Close()

	body := `{"target":"` + target.URL + `","workers":2,"maxRequests":10,"payloadSize":{"value":"256B"}}`
	resp, err := http.Post(api.URL+"/runs", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var created struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.ID == "" {
		t.Fatal("missing id")
	}

	// Poll GET /runs/{id} until completed.
	var snap struct {
		State   string `json:"state"`
		Metrics struct {
			Completed int `json:"completed"`
		} `json:"metrics"`
	}
	for range 100 {
		r, _ := http.Get(api.URL + "/runs/" + created.ID)
		json.NewDecoder(r.Body).Decode(&snap)
		r.Body.Close()
		if snap.State == "completed" {
			break
		}
	}
	if snap.State != "completed" {
		t.Fatalf("state = %s, want completed", snap.State)
	}
	if snap.Metrics.Completed != 10 {
		t.Errorf("completed = %d, want 10", snap.Metrics.Completed)
	}
}

func TestAPICreateRunValidationError(t *testing.T) {
	api := httptest.NewServer(newAPI(t))
	defer api.Close()
	resp, _ := http.Post(api.URL+"/runs", "application/json", strings.NewReader(`{"workers":1}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestAPIListRuns(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(handler))
	defer target.Close()
	api := httptest.NewServer(newAPI(t))
	defer api.Close()

	body := `{"target":"` + target.URL + `","workers":1,"maxRequests":5,"payloadSize":{"value":"64B"}}`
	http.Post(api.URL+"/runs", "application/json", strings.NewReader(body))

	resp, _ := http.Get(api.URL + "/runs")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var list []map[string]any
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 1 {
		t.Errorf("list len = %d, want 1", len(list))
	}
}

func TestAPIStopRun(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(handler))
	defer target.Close()
	api := httptest.NewServer(newAPI(t))
	defer api.Close()

	body := `{"target":"` + target.URL + `","workers":1,"duration":"60s","payloadSize":{"value":"64B"}}`
	resp, _ := http.Post(api.URL+"/runs", "application/json", strings.NewReader(body))
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	stopResp, _ := http.Post(api.URL+"/runs/"+created.ID+"/stop", "", nil)
	if stopResp.StatusCode != http.StatusOK {
		t.Errorf("stop status = %d, want 200", stopResp.StatusCode)
	}
	stopResp.Body.Close()

	stopMissing, _ := http.Post(api.URL+"/runs/nope/stop", "", nil)
	if stopMissing.StatusCode != http.StatusNotFound {
		t.Errorf("stop missing status = %d, want 404", stopMissing.StatusCode)
	}
	stopMissing.Body.Close()
}

func TestAPIGetUnknown(t *testing.T) {
	api := httptest.NewServer(newAPI(t))
	defer api.Close()
	resp, _ := http.Get(api.URL + "/runs/nope")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}
