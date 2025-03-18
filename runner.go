package testkit

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	_ "github.com/lib/pq" // Import the PostgreSQL driver
)

// DefaultTimeout is the default timeout for HTTP requests
const DefaultTimeout = time.Second * 10

// Global runner instance that can be accessed by tests
var Runner *TestRunner

// AppStarter defines the interface for starting and stopping an application
type AppStarter interface {
	// Start starts the application
	Start() error
	// Stop stops the application
	Stop(ctx context.Context) error
}

// RunnerConfig holds configuration for the test runner
type RunnerConfig struct {
	// Database connection string
	DBConnectionString string
	// Base URL for the API
	BaseURL string
	// Path to fixtures directory
	FixturesDir string
	// Application to start
	App AppStarter
	// Health check endpoint path (defaults to "/v1/health/liveness")
	HealthCheckPath string
	// Maximum number of attempts to wait for server (defaults to 30)
	MaxWaitAttempts int
}

// TestRunner manages the test environment and execution
type TestRunner struct {
	config         *RunnerConfig
	db             *sql.DB
	httpClient     *http.Client
	fixtureManager *FixtureManager
	cleanup        func()
}

// RunWithTesting runs tests with the given testing.M and configuration
// This is a convenience function that handles creating the runner, running tests, and cleanup
func RunWithTesting(m *testing.M, config *RunnerConfig) {
	// Create runner
	var err error
	Runner, err = NewTestRunner(config)
	if err != nil {
		panic(fmt.Errorf("failed to create test runner: %w", err))
	}
	defer Runner.Cleanup()

	// Run tests
	code := Runner.Run(m)

	// Exit with the test result code
	if code != 0 {
		panic("Tests failed")
	}
}

// NewTestRunner creates a new test runner with the given configuration
func NewTestRunner(config *RunnerConfig) (*TestRunner, error) {
	// Set defaults for optional fields
	if config.HealthCheckPath == "" {
		config.HealthCheckPath = "/v1/health/liveness"
	}
	if config.MaxWaitAttempts <= 0 {
		config.MaxWaitAttempts = 30
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: DefaultTimeout,
	}

	// Connect to database
	db, err := sql.Open("postgres", config.DBConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Initialize fixture manager
	fixtureManager := NewFixtureManager(db)

	// Create test runner
	runner := &TestRunner{
		config:         config,
		db:             db,
		httpClient:     client,
		fixtureManager: fixtureManager,
		cleanup: func() {
			if fixtureManager != nil {
				if err := fixtureManager.CleanupFixtures(); err != nil {
					log.Printf("Warning: failed to cleanup fixtures: %v", err)
				}
			}
			if db != nil {
				if err := db.Close(); err != nil {
					log.Printf("Warning: failed to close database connection: %v", err)
				}
			}
			if config.App != nil {
				if err := config.App.Stop(context.Background()); err != nil {
					log.Printf("Warning: failed to stop application: %v", err)
				}
			}
		},
	}

	// Start the application if provided
	if config.App != nil {
		// Start application in a goroutine
		go func() {
			if err := config.App.Start(); err != nil {
				log.Panic(fmt.Errorf("failed to start application: %w", err))
			}
		}()

		// Wait for the server to be ready
		healthCheckURL := fmt.Sprintf("%s%s", config.BaseURL, config.HealthCheckPath)
		if err := runner.waitForServer(healthCheckURL, config.MaxWaitAttempts); err != nil {
			runner.Cleanup()
			return nil, fmt.Errorf("server did not start in time: %w", err)
		}
	}

	return runner, nil
}

// LoadFixtures loads fixtures from the specified directory
func (r *TestRunner) LoadFixtures() error {
	return r.fixtureManager.LoadFixturesFromDir(r.config.FixturesDir)
}

// Run runs the tests using the provided testing.M
func (r *TestRunner) Run(m *testing.M) int {
	// Load fixtures
	if err := r.LoadFixtures(); err != nil {
		log.Fatalf("Failed to load fixtures: %v", err)
	}

	// Run tests
	return m.Run()
}

// WaitForServer checks if the server is ready at the specified URL
func (r *TestRunner) waitForServer(url string, maxAttempts int) error {
	for i := range maxAttempts {
		log.Printf("Waiting for server to be ready at %s (attempt %d/%d)", url, i+1, maxAttempts)

		// Create a context with timeout for the request
		ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		if err != nil {
			cancel()
			return fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := r.httpClient.Do(req)
		cancel() // Always cancel the context to release resources

		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			log.Printf("Server is ready at %s", url)
			return nil
		}
		if err == nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("server did not respond after %d attempts", maxAttempts)
}

// Cleanup cleans up resources used by the test runner
func (r *TestRunner) Cleanup() {
	if r.cleanup != nil {
		r.cleanup()
	}
}

// GetHTTPClient returns the HTTP client
func (r *TestRunner) GetHTTPClient() *http.Client {
	return r.httpClient
}

// GetBaseURL returns the base URL for the test server
func (r *TestRunner) GetBaseURL() string {
	return r.config.BaseURL
}

// GetFixtureManager returns the fixture manager
func (r *TestRunner) GetFixtureManager() *FixtureManager {
	return r.fixtureManager
}

// GetDB returns the database connection
func (r *TestRunner) GetDB() *sql.DB {
	return r.db
}

// GetConfig returns the configuration
func (r *TestRunner) GetConfig() *RunnerConfig {
	return r.config
}
