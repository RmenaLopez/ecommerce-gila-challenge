package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port        string
	DatabaseDSN string
}

func Load() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return &Config{Port: port, DatabaseDSN: dsn}, nil
}
