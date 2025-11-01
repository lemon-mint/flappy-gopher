package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gosuda/portal/sdk"
	"github.com/julienschmidt/httprouter"
)

// Score represents a player's score entry
type Score struct {
	Name      string    `json:"name"`
	Score     int       `json:"score"`
	Timestamp time.Time `json:"timestamp"`
}

// Leaderboard manages the score entries
type Leaderboard struct {
	mu      sync.RWMutex
	entries []Score
}

var leaderboard = &Leaderboard{
	entries: make([]Score, 0),
}

// AddScore adds a new score to the leaderboard
func (lb *Leaderboard) AddScore(name string, score int) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	entry := Score{
		Name:      name,
		Score:     score,
		Timestamp: time.Now(),
	}

	lb.entries = append(lb.entries, entry)

	// Sort by score (descending)
	sort.Slice(lb.entries, func(i, j int) bool {
		return lb.entries[i].Score > lb.entries[j].Score
	})

	// Keep only top 10
	if len(lb.entries) > 10 {
		lb.entries = lb.entries[:10]
	}
}

// GetTopScores returns the top scores
func (lb *Leaderboard) GetTopScores() []Score {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make([]Score, len(lb.entries))
	copy(result, lb.entries)
	return result
}

// handleSubmitScore handles POST /api/scores
func handleSubmitScore(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name  string `json:"name"`
		Score int    `json:"score"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	if req.Score < 0 {
		http.Error(w, "Invalid score", http.StatusBadRequest)
		return
	}

	leaderboard.AddScore(req.Name, req.Score)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// handleGetLeaderboard handles GET /api/leaderboard
func handleGetLeaderboard(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	scores := leaderboard.GetTopScores()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scores)
}

func main() {
	client, err := sdk.NewClient(
		sdk.WithBootstrapServers([]string{
			"wss://portal.gosuda.org/relay",
			"ws://localhost:4017/relay",
		}),
	)
	if err != nil {
		panic(err)
	}

	cred := sdk.NewCredential()
	ln, err := client.Listen(cred, "Flappy-Gopher", []string{"http/1.1"})
	if err != nil {
		panic(err)
	}

	r := httprouter.New()

	// API endpoints
	r.POST("/api/scores", handleSubmitScore)
	r.GET("/api/leaderboard", handleGetLeaderboard)

	// Static files
	r.NotFound = http.FileServer(http.Dir("./web"))

	http.Serve(ln, r)
}
