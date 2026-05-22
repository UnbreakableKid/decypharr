package arr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirrobot01/decypharr/internal/config"
)

func TestIsUsenetSample(t *testing.T) {
	a := &Arr{}


	tests := []struct {
		name     string
		protocol string
		messages []struct {
			Title    string   `json:"title"`
			Messages []string `json:"messages"`
		}
		expected bool
	}{
		{
			name:     "Non-usenet sample",
			protocol: "torrent",
			messages: []struct {
				Title    string   `json:"title"`
				Messages []string `json:"messages"`
			}{
				{Title: "Sample", Messages: []string{"File is a sample"}},
			},
			expected: false,
		},
		{
			name:     "Usenet clean item",
			protocol: "usenet",
			messages: nil,
			expected: false,
		},
		{
			name:     "Usenet title sample",
			protocol: "usenet",
			messages: []struct {
				Title    string   `json:"title"`
				Messages []string `json:"messages"`
			}{
				{Title: "Sample", Messages: []string{"Some warning message"}},
			},
			expected: true,
		},
		{
			name:     "Usenet message sample",
			protocol: "usenet",
			messages: []struct {
				Title    string   `json:"title"`
				Messages []string `json:"messages"`
			}{
				{Title: "Warning", Messages: []string{"Unable to determine if sample"}},
			},
			expected: true,
		},
		{
			name:     "Usenet lowercase sample",
			protocol: "usenet",
			messages: []struct {
				Title    string   `json:"title"`
				Messages []string `json:"messages"`
			}{
				{Title: "Warning", Messages: []string{"some sample file detected"}},
			},
			expected: true,
		},
		{
			name:     "Usenet unrelated warning",
			protocol: "Usenet",
			messages: []struct {
				Title    string   `json:"title"`
				Messages []string `json:"messages"`
			}{
				{Title: "Warning", Messages: []string{"Slow download speed"}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := QueueSchema{
				Protocol: tt.protocol,
			}
			for _, m := range tt.messages {
				q.StatusMessages = append(q.StatusMessages, struct {
					Title    string   `json:"title"`
					Messages []string `json:"messages"`
				}{
					Title:    m.Title,
					Messages: m.Messages,
				})
			}

			actual := a.isUsenetSample(q)
			if actual != tt.expected {
				t.Errorf("expected isUsenetSample to be %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestCleanupQueueWithUsenetSample(t *testing.T) {
	config.SetConfigPath(t.TempDir())
	// Set up httptest server to verify the action requests are correctly made
	var bulkDeletedCount int
	var bulkDeletedPayload struct {
		Ids []int `json:"ids"`
	}
	var queryParams struct {
		RemoveFromClient string
		Blocklist        string
		SkipRedownload   string
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/queue" && r.Method == http.MethodGet {
			// Return some queue items
			qItems := []QueueSchema{
				{
					Id:       101,
					Protocol: "usenet",
					StatusMessages: []struct {
						Title    string   `json:"title"`
						Messages []string `json:"messages"`
					}{
						{Title: "Warning", Messages: []string{"Unable to determine if sample"}},
					},
				},
				{
					Id:       102,
					Protocol: "usenet",
					StatusMessages: []struct {
						Title    string   `json:"title"`
						Messages []string `json:"messages"`
					}{
						{Title: "Sample", Messages: []string{"File is a sample"}},
					},
				},
				{
					Id:       103,
					Protocol: "torrent", // Should not match Usenet sample filter
					StatusMessages: []struct {
						Title    string   `json:"title"`
						Messages []string `json:"messages"`
					}{
						{Title: "Sample", Messages: []string{"File is a sample"}},
					},
				},
			}
			data := QueueResponseScheme{
				Records: qItems,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(data)
			return
		}

		if r.URL.Path == "/api/v3/queue/bulk" && r.Method == http.MethodDelete {
			bulkDeletedCount++
			queryParams.RemoveFromClient = r.URL.Query().Get("removeFromClient")
			queryParams.Blocklist = r.URL.Query().Get("blocklist")
			queryParams.SkipRedownload = r.URL.Query().Get("skipRedownload")
			json.NewDecoder(r.Body).Decode(&bulkDeletedPayload)
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Test 1: sample_action = "fail", unable_to_determine_action = "fail"
	bulkDeletedCount = 0
	a := New("sonarr", server.URL, "dummy-token", true, false, nil, "", "manual")
	a.SampleAction = "fail"
	a.UnableToDetermineAction = "fail"

	err := a.CleanupQueue()
	if err != nil {
		t.Fatalf("CleanupQueue failed: %v", err)
	}

	if bulkDeletedCount != 1 {
		t.Errorf("expected 1 bulk delete call, got %d", bulkDeletedCount)
	}
	if queryParams.RemoveFromClient != "true" || queryParams.Blocklist != "false" || queryParams.SkipRedownload != "false" {
		t.Errorf("unexpected query params: %+v", queryParams)
	}
	// We expect 101 and 102 to be deleted. 103 is torrent, so it's ignored.
	if len(bulkDeletedPayload.Ids) != 2 || bulkDeletedPayload.Ids[0] != 101 || bulkDeletedPayload.Ids[1] != 102 {
		t.Errorf("expected IDs [101, 102], got %v", bulkDeletedPayload.Ids)
	}

	// Test 2: sample_action = "fail", unable_to_determine_action = ""
	bulkDeletedCount = 0
	a.SampleAction = "fail"
	a.UnableToDetermineAction = ""
	err = a.CleanupQueue()
	if err != nil {
		t.Fatalf("CleanupQueue failed: %v", err)
	}
	if bulkDeletedCount != 1 {
		t.Errorf("expected 1 bulk delete call, got %d", bulkDeletedCount)
	}
	// We expect only 102 to be deleted.
	if len(bulkDeletedPayload.Ids) != 1 || bulkDeletedPayload.Ids[0] != 102 {
		t.Errorf("expected IDs [102], got %v", bulkDeletedPayload.Ids)
	}

	// Test 3: sample_action = "", unable_to_determine_action = "fail"
	bulkDeletedCount = 0
	a.SampleAction = ""
	a.UnableToDetermineAction = "fail"
	err = a.CleanupQueue()
	if err != nil {
		t.Fatalf("CleanupQueue failed: %v", err)
	}
	if bulkDeletedCount != 1 {
		t.Errorf("expected 1 bulk delete call, got %d", bulkDeletedCount)
	}
	// We expect only 101 to be deleted.
	if len(bulkDeletedPayload.Ids) != 1 || bulkDeletedPayload.Ids[0] != 101 {
		t.Errorf("expected IDs [101], got %v", bulkDeletedPayload.Ids)
	}

	// Test 4: sample_action = "fail_blocklist", unable_to_determine_action = "fail_blocklist"
	bulkDeletedCount = 0
	a.SampleAction = "fail_blocklist"
	a.UnableToDetermineAction = "fail_blocklist"
	err = a.CleanupQueue()
	if err != nil {
		t.Fatalf("CleanupQueue failed: %v", err)
	}
	if bulkDeletedCount != 1 {
		t.Errorf("expected 1 bulk delete call, got %d", bulkDeletedCount)
	}
	if queryParams.RemoveFromClient != "true" || queryParams.Blocklist != "true" || queryParams.SkipRedownload != "false" {
		t.Errorf("unexpected query params for fail and blocklist: %+v", queryParams)
	}
}
