// Command m-seed populates the database with demo data.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropics/m/internal/store"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Config represents the server configuration file (same as m-server).
type Config struct {
	Server struct {
		Port   int    `yaml:"port"`
		APIKey string `yaml:"api_key"`
	} `yaml:"server"`
	Storage struct {
		Path string `yaml:"path"`
	} `yaml:"storage"`
	Workspaces struct {
		Path string `yaml:"path"`
	} `yaml:"workspaces"`
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("Loading database: %s", cfg.Storage.Path)

	// Initialize store
	s, err := store.New(cfg.Storage.Path)
	if err != nil {
		log.Fatalf("failed to initialize store: %v", err)
	}
	defer s.Close()

	log.Println("Clearing existing data...")
	if err := clearDatabase(s); err != nil {
		log.Fatalf("failed to clear database: %v", err)
	}

	log.Println("Creating demo repositories...")
	repos, err := createDemoRepos(s)
	if err != nil {
		log.Fatalf("failed to create repos: %v", err)
	}

	log.Println("Creating demo runs...")
	if err := createDemoRuns(s, repos); err != nil {
		log.Fatalf("failed to create runs: %v", err)
	}

	log.Println("✓ Demo data seeded successfully!")
	log.Printf("  - %d repositories", len(repos))
	log.Println("  - Multiple runs with different states")
	log.Println("  - Pending approval (triggers iOS banner)")
	log.Println("  - In-progress run (shows live updates)")
}

func loadConfig(path string) (*Config, error) {
	cfg := &Config{}

	// Set defaults (same as m-server)
	cfg.Server.Port = 8080
	cfg.Storage.Path = "./data/m.db"
	cfg.Workspaces.Path = "./workspaces"

	// Load config file if it exists
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Environment variable overrides
	if v := os.Getenv("M_DB_PATH"); v != "" {
		cfg.Storage.Path = v
	}

	return cfg, nil
}

// clearDatabase deletes all data in the correct order (respects foreign keys).
func clearDatabase(s *store.Store) error {
	db := s.DB()

	// Order matters: delete children before parents
	tables := []string{
		"interactions",
		"approvals",
		"events",
		"runs",
		"repos",
		"devices",
	}

	for _, table := range tables {
		if _, err := db.Exec(fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			return fmt.Errorf("delete from %s: %w", table, err)
		}
	}

	return nil
}

// createDemoRepos creates demo repositories.
func createDemoRepos(s *store.Store) (map[string]*store.Repo, error) {
	repos := make(map[string]*store.Repo)

	// Repo 1: demo-web-app
	webAppURL := "https://github.com/demo/web-app"
	webApp, err := s.CreateRepo("demo-web-app", &webAppURL)
	if err != nil {
		return nil, fmt.Errorf("create demo-web-app: %w", err)
	}
	repos["web-app"] = webApp

	// Repo 2: demo-api
	apiURL := "https://github.com/demo/api"
	api, err := s.CreateRepo("demo-api", &apiURL)
	if err != nil {
		return nil, fmt.Errorf("create demo-api: %w", err)
	}
	repos["api"] = api

	return repos, nil
}

// createDemoRuns creates runs with various states and realistic events.
func createDemoRuns(s *store.Store, repos map[string]*store.Repo) error {
	now := time.Now()

	// Run 1: Completed success (demo-web-app, 2 hours ago)
	if err := createCompletedSuccessRun(s, repos["web-app"], now.Add(-2*time.Hour)); err != nil {
		return fmt.Errorf("create completed success run: %w", err)
	}

	// Run 2: Failed run (demo-web-app, 1 hour ago)
	if err := createFailedRun(s, repos["web-app"], now.Add(-1*time.Hour)); err != nil {
		return fmt.Errorf("create failed run: %w", err)
	}

	// Run 3: Waiting approval (demo-web-app, 30 min ago)
	if err := createWaitingApprovalRun(s, repos["web-app"], now.Add(-30*time.Minute)); err != nil {
		return fmt.Errorf("create waiting approval run: %w", err)
	}

	// Run 4: Completed success (demo-api, 45 min ago)
	if err := createQuickSuccessRun(s, repos["api"], now.Add(-45*time.Minute)); err != nil {
		return fmt.Errorf("create quick success run: %w", err)
	}

	// Run 5: In-progress (demo-api, 5 min ago)
	if err := createInProgressRun(s, repos["api"], now.Add(-5*time.Minute)); err != nil {
		return fmt.Errorf("create in-progress run: %w", err)
	}

	return nil
}

// createCompletedSuccessRun creates a completed successful run with realistic events.
func createCompletedSuccessRun(s *store.Store, repo *store.Repo, startTime time.Time) error {
	runID := uuid.New().String()
	workspacePath := filepath.Join("./workspaces", runID)

	// Create run (we'll update state later)
	run, err := s.CreateRunWithID(runID, repo.ID, "Fix CSS styling bug in header component", workspacePath)
	if err != nil {
		return err
	}

	// Update run timestamps to match startTime
	if err := updateRunTimestamps(s, run.ID, startTime); err != nil {
		return err
	}

	// Create events with realistic timing
	events := []struct {
		offset time.Duration
		typ    string
		data   string
	}{
		{0, "stdout", "Starting task..."},
		{10 * time.Millisecond, "stdout", "Analyzing codebase..."},
		{50 * time.Millisecond, "tool_start", `{"call_id":"call-1","tool":"Read","input":{"file_path":"src/components/Header.css"}}`},
		{100 * time.Millisecond, "tool_end", `{"call_id":"call-1","tool":"Read","success":true,"duration_ms":95}`},
		{20 * time.Millisecond, "stdout", "Found styling issue in Header.css"},
		{30 * time.Millisecond, "tool_start", `{"call_id":"call-2","tool":"Edit","input":{"file_path":"src/components/Header.css","old_string":".header { margin: 10px }","new_string":".header { margin: 0 }"}}`},
		{80 * time.Millisecond, "tool_end", `{"call_id":"call-2","tool":"Edit","success":true,"duration_ms":75}`},
		{20 * time.Millisecond, "stdout", "CSS fix applied successfully"},
		{10 * time.Millisecond, "run_completed", `{"message":"Task completed successfully"}`},
	}

	currentTime := startTime
	for _, evt := range events {
		currentTime = currentTime.Add(evt.offset)
		if err := createEventAt(s, run.ID, evt.typ, evt.data, currentTime); err != nil {
			return err
		}
	}

	// Mark as completed
	return s.UpdateRunState(run.ID, store.RunStateCompleted)
}

// createFailedRun creates a failed run with error events.
func createFailedRun(s *store.Store, repo *store.Repo, startTime time.Time) error {
	runID := uuid.New().String()
	workspacePath := filepath.Join("./workspaces", runID)

	run, err := s.CreateRunWithID(runID, repo.ID, "Add authentication system", workspacePath)
	if err != nil {
		return err
	}

	if err := updateRunTimestamps(s, run.ID, startTime); err != nil {
		return err
	}

	events := []struct {
		offset time.Duration
		typ    string
		data   string
	}{
		{0, "stdout", "Starting authentication implementation..."},
		{20 * time.Millisecond, "tool_start", `{"call_id":"call-1","tool":"Glob","input":{"pattern":"**/auth/*.go"}}`},
		{50 * time.Millisecond, "tool_end", `{"call_id":"call-1","tool":"Glob","success":true,"duration_ms":45}`},
		{30 * time.Millisecond, "stderr", "Error: auth package not found"},
		{10 * time.Millisecond, "stderr", "Cannot proceed without existing auth structure"},
		{10 * time.Millisecond, "run_failed", `{"error":"Missing required auth package"}`},
	}

	currentTime := startTime
	for _, evt := range events {
		currentTime = currentTime.Add(evt.offset)
		if err := createEventAt(s, run.ID, evt.typ, evt.data, currentTime); err != nil {
			return err
		}
	}

	return s.UpdateRunState(run.ID, store.RunStateFailed)
}

// createWaitingApprovalRun creates a run with pending approval (triggers iOS banner).
func createWaitingApprovalRun(s *store.Store, repo *store.Repo, startTime time.Time) error {
	runID := uuid.New().String()
	workspacePath := filepath.Join("./workspaces", runID)

	run, err := s.CreateRunWithID(runID, repo.ID, "Update npm dependencies", workspacePath)
	if err != nil {
		return err
	}

	if err := updateRunTimestamps(s, run.ID, startTime); err != nil {
		return err
	}

	events := []struct {
		offset time.Duration
		typ    string
		data   string
	}{
		{0, "stdout", "Checking package.json..."},
		{30 * time.Millisecond, "tool_start", `{"call_id":"call-1","tool":"Read","input":{"file_path":"package.json"}}`},
		{60 * time.Millisecond, "tool_end", `{"call_id":"call-1","tool":"Read","success":true,"duration_ms":55}`},
		{20 * time.Millisecond, "stdout", "Found outdated dependencies"},
		{40 * time.Millisecond, "tool_start", `{"call_id":"call-2","tool":"Edit","input":{"file_path":"package.json"}}`},
		{10 * time.Millisecond, "approval_requested", `{"type":"diff","tool":"Edit"}`},
	}

	currentTime := startTime
	var lastEventID string
	for _, evt := range events {
		currentTime = currentTime.Add(evt.offset)
		event, err := createEventAtWithID(s, run.ID, evt.typ, evt.data, currentTime)
		if err != nil {
			return err
		}
		lastEventID = event.ID
	}

	// Create pending approval
	payload := `{"file_path":"package.json","diff":"--- package.json\n+++ package.json\n@@ -5,7 +5,7 @@\n   \"dependencies\": {\n-    \"react\": \"^17.0.0\",\n+    \"react\": \"^18.2.0\",\n-    \"react-dom\": \"^17.0.0\"\n+    \"react-dom\": \"^18.2.0\"\n   }\n }"}`
	if _, err := s.CreateApproval(run.ID, lastEventID, store.ApprovalTypeDiff, &payload); err != nil {
		return err
	}

	return s.UpdateRunState(run.ID, store.RunStateWaitingApproval)
}

// createQuickSuccessRun creates a quick successful run.
func createQuickSuccessRun(s *store.Store, repo *store.Repo, startTime time.Time) error {
	runID := uuid.New().String()
	workspacePath := filepath.Join("./workspaces", runID)

	run, err := s.CreateRunWithID(runID, repo.ID, "Add logging endpoint", workspacePath)
	if err != nil {
		return err
	}

	if err := updateRunTimestamps(s, run.ID, startTime); err != nil {
		return err
	}

	events := []struct {
		offset time.Duration
		typ    string
		data   string
	}{
		{0, "stdout", "Adding logging endpoint..."},
		{20 * time.Millisecond, "tool_start", `{"call_id":"call-1","tool":"Write","input":{"file_path":"api/logging.go"}}`},
		{100 * time.Millisecond, "tool_end", `{"call_id":"call-1","tool":"Write","success":true,"duration_ms":95}`},
		{15 * time.Millisecond, "stdout", "Logging endpoint created"},
		{10 * time.Millisecond, "run_completed", `{"message":"Task completed"}`},
	}

	currentTime := startTime
	for _, evt := range events {
		currentTime = currentTime.Add(evt.offset)
		if err := createEventAt(s, run.ID, evt.typ, evt.data, currentTime); err != nil {
			return err
		}
	}

	return s.UpdateRunState(run.ID, store.RunStateCompleted)
}

// createInProgressRun creates an in-progress run (shows live updates in iOS).
func createInProgressRun(s *store.Store, repo *store.Repo, startTime time.Time) error {
	runID := uuid.New().String()
	workspacePath := filepath.Join("./workspaces", runID)

	run, err := s.CreateRunWithID(runID, repo.ID, "Refactor database layer", workspacePath)
	if err != nil {
		return err
	}

	if err := updateRunTimestamps(s, run.ID, startTime); err != nil {
		return err
	}

	events := []struct {
		offset time.Duration
		typ    string
		data   string
	}{
		{0, "stdout", "Starting database refactor..."},
		{30 * time.Millisecond, "stdout", "Analyzing current structure..."},
		{100 * time.Millisecond, "tool_start", `{"call_id":"call-1","tool":"Glob","input":{"pattern":"**/db/*.go"}}`},
		{80 * time.Millisecond, "tool_end", `{"call_id":"call-1","tool":"Glob","success":true,"duration_ms":75}`},
		{50 * time.Millisecond, "stdout", "Found 5 database files"},
		{40 * time.Millisecond, "tool_start", `{"call_id":"call-2","tool":"Read","input":{"file_path":"db/queries.go"}}`},
		// Note: Intentionally no tool_end - shows in-progress tool call
	}

	currentTime := startTime
	for _, evt := range events {
		currentTime = currentTime.Add(evt.offset)
		if err := createEventAt(s, run.ID, evt.typ, evt.data, currentTime); err != nil {
			return err
		}
	}

	// Leave in running state (no UpdateRunState call)
	return nil
}

// createEventAt creates an event at a specific time.
func createEventAt(s *store.Store, runID, eventType, data string, at time.Time) error {
	_, err := createEventAtWithID(s, runID, eventType, data, at)
	return err
}

// createEventAtWithID creates an event at a specific time and returns it.
func createEventAtWithID(s *store.Store, runID, eventType, data string, at time.Time) (*store.Event, error) {
	// Use direct DB access to set custom timestamp
	db := s.DB()

	id := uuid.New().String()

	// Get next sequence number
	var nullSeq sql.NullInt64
	if err := db.QueryRow("SELECT MAX(seq) FROM events WHERE run_id = ?", runID).Scan(&nullSeq); err != nil {
		return nil, fmt.Errorf("get max seq: %w", err)
	}

	nextSeq := int64(1)
	if nullSeq.Valid {
		nextSeq = nullSeq.Int64 + 1
	}

	dataPtr := &data
	if data == "" {
		dataPtr = nil
	}

	_, err := db.Exec(
		"INSERT INTO events (id, run_id, seq, type, data, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, runID, nextSeq, eventType, dataPtr, at.Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("insert event: %w", err)
	}

	return &store.Event{
		ID:        id,
		RunID:     runID,
		Seq:       nextSeq,
		Type:      eventType,
		Data:      dataPtr,
		CreatedAt: at,
	}, nil
}

// updateRunTimestamps updates a run's created_at and updated_at timestamps.
func updateRunTimestamps(s *store.Store, runID string, t time.Time) error {
	db := s.DB()
	_, err := db.Exec(
		"UPDATE runs SET created_at = ?, updated_at = ? WHERE id = ?",
		t.Unix(), t.Unix(), runID,
	)
	return err
}
