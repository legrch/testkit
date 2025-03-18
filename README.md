# Go Testing Toolkit

[![Go Reference](https://pkg.go.dev/badge/github.com/legrch/testkit.svg)](https://pkg.go.dev/github.com/legrch/testkit)
[![Go Report Card](https://goreportcard.com/badge/github.com/legrch/testkit)](https://goreportcard.com/report/github.com/legrch/testkit)
[![License](https://img.shields.io/github/license/legrch/testkit)](LICENSE)
[![Release](https://img.shields.io/github/v/release/legrch/testkit)](https://github.com/legrch/testkit/releases)

A Go toolkit for simplifying testing, particularly for integration tests with external services and databases.

## Features

- **Database fixtures**: Easily load and reset test data
- **Test runners**: Simplified test setup and teardown
- **Environment management**: Load environment variables for tests
- **SQL helpers**: Utilities for database testing
- **Test lifecycle management**: Coordinate test setup and cleanup

## Installation

```bash
go get github.com/legrch/testkit
```

## Quick Start

### Database Test Fixtures

```go
package example_test

import (
	"testing"

	"github.com/legrch/testkit"
)

func TestWithFixtures(t *testing.T) {
	// Load fixtures from JSON files
	fixtures, err := testkit.LoadFixtures("testdata/fixtures")
	if err != nil {
		t.Fatalf("Failed to load fixtures: %v", err)
	}

	// Connect to test database
	db, err := testkit.Connect()
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Apply fixtures to database
	if err := fixtures.Apply(db); err != nil {
		t.Fatalf("Failed to apply fixtures: %v", err)
	}

	// Run tests with loaded fixtures
	// ...

	// Clean up fixtures
	if err := fixtures.Reset(db); err != nil {
		t.Fatalf("Failed to reset fixtures: %v", err)
	}
}
```

### Environment Variable Management

```go
package example_test

import (
	"testing"

	"github.com/legrch/testkit"
)

func TestWithEnvironment(t *testing.T) {
	// Load .env.test file for testing
	env, err := testkit.LoadTestEnv(".env.test")
	if err != nil {
		t.Fatalf("Failed to load test environment: %v", err)
	}
	defer env.Restore()

	// Run tests with environment variables
	// ...
}
```

## Documentation

For detailed documentation, examples, and API reference, please visit:

- [Package Documentation](https://pkg.go.dev/github.com/legrch/testkit)
- [Examples](https://github.com/legrch/testkit/tree/main/examples)
- [API Reference](https://pkg.go.dev/github.com/legrch/testkit/pkg/testkit)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details. 