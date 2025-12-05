package config

import (
	"os"
)

type Config struct {
	NATSURL       string
	ClickHouseURL string
	Database      string
}

func Load() *Config {
	return &Config{
		NATSURL:       getEnv("NATS_URL", "nats://localhost:4222"),
		ClickHouseURL: getEnv("CLICKHOUSE_URL", "tcp://localhost:9000"),
		Database:      getEnv("CLICKHOUSE_DATABASE", "default"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
