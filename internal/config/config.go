package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Config holds application configuration.
type Config struct {
	GRPCPort           string
	DatabaseURL        string
	DatabaseReaderURL  string
	DatabaseWriterURL  string
	LiquibaseChangelog string
}

// Load reads config from app.properties, then overrides with environment variables.
// Environment variables take precedence over the properties file.
func Load() (*Config, error) {
	props := loadProperties("app.properties")

	dbURL := envOrProp("DATABASE_URL", "database_url", props)
	if dbURL == "" {
		return nil, fmt.Errorf("database_url is required (set in app.properties or DATABASE_URL env var)")
	}

	dbReaderURL := envOrProp("DATABASE_READER_URL", "database_reader_url", props)
	if dbReaderURL == "" {
		dbReaderURL = dbURL // fallback to primary if no reader configured
	}

	dbWriterURL := envOrProp("DATABASE_WRITER_URL", "database_writer_url", props)
	if dbWriterURL == "" {
		dbWriterURL = dbURL // fallback to primary if no writer configured
	}

	grpcPort := envOrProp("GRPC_PORT", "grpc_port", props)
	if grpcPort == "" {
		grpcPort = "50051"
	}

	lbChangelog := envOrProp("LIQUIBASE_CHANGELOG", "liquibase_changelog", props)
	if lbChangelog == "" {
		lbChangelog = "db/changelog-master.yaml"
	}

	return &Config{
		GRPCPort:           grpcPort,
		DatabaseURL:        dbURL,
		DatabaseReaderURL:  dbReaderURL,
		DatabaseWriterURL:  dbWriterURL,
		LiquibaseChangelog: lbChangelog,
	}, nil
}

// envOrProp returns the environment variable value if set, otherwise the properties file value.
func envOrProp(envKey, propKey string, props map[string]string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return props[propKey]
}

// loadProperties reads a key=value properties file into a map.
func loadProperties(path string) map[string]string {
	props := make(map[string]string)

	f, err := os.Open(path)
	if err != nil {
		return props
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		props[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	return props
}
