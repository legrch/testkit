# TestKit Package

This package provides utilities for testing Go applications, with a focus on integration testing.

## Components

### Fixtures

The fixtures module provides functions for loading and managing database fixtures:

```go
fixtures, err := testkit.LoadFixtures("testdata/fixtures")
if err != nil {
    t.Fatalf("Failed to load fixtures: %v", err)
}

// Apply fixtures to database
if err := fixtures.Apply(db); err != nil {
    t.Fatalf("Failed to apply fixtures: %v", err)
}
```

### Test Runner

The runner module provides a way to run integration tests with proper setup and teardown:

```go
runner := testkit.NewRunner(&testkit.RunnerConfig{
    DatabaseURL: "postgres://user:password@localhost:5432/testdb",
    FixturesDir: "testdata/fixtures",
})

runner.Run(t, func(t *testing.T, db *sql.DB) {
    // Run tests with configured database and fixtures
})
```

### Environment

The env module provides utilities for managing environment variables during tests:

```go
env, err := testkit.LoadTestEnv(".env.test")
if err != nil {
    t.Fatalf("Failed to load test environment: %v", err)
}
defer env.Restore()
```

## Best Practices

- Use fixtures to ensure consistent test data
- Clean up after tests to maintain isolation
- Use environment variables to configure test behavior
- Keep tests independent of each other 