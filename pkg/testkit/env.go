package testkit

import (
	"github.com/joho/godotenv"

	"github.com/legrch/logger/pkg/logger"
)

// LoadEnvFiles loads environment variables from the specified files
// The files are loaded in order, with later files taking precedence over earlier ones
func LoadEnvFiles(envFiles ...string) {
	for i, file := range envFiles {
		if file == "" {
			continue
		}

		// Use Overload for all files except the first one to ensure later files take precedence
		var err error
		if i == 0 {
			err = godotenv.Load(file)
		} else {
			err = godotenv.Overload(file)
		}

		if err != nil {
			logger.Warn("Failed to load env file", "file", file, "error", err)
		}
	}
}
