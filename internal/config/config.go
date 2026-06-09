// Package config loads runtime settings from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL    string
	HTTPAddr       string
	PublicURL      string
	ServerName     string
	ServerDesc     string
	ServerMaxUsers int
	JWTSecret      string
	JWTTTL         time.Duration
	BcryptCost     int
	SchedulerTick  time.Duration
	LogLevel       string
	LogFormat      string

	DefaultUniverseName          string
	DefaultUniverseGalaxies      int
	DefaultUniverseSystems       int
	DefaultUniverseSpeedEconomy  int
	DefaultUniverseSpeedFleet    int
	DefaultUniverseSpeedResearch int
}

func Load() (*Config, error) {
	c := &Config{
		DatabaseURL:                  getenv("DATABASE_URL", "postgres://tarmy:tarmy@localhost:5432/tarmy?sslmode=disable"),
		HTTPAddr:                     getenv("TARMY_HTTP_ADDR", "0.0.0.0:8080"),
		PublicURL:                    getenv("TARMY_PUBLIC_URL", "http://localhost:8080"),
		ServerName:                   getenv("TARMY_SERVER_NAME", "s1"),
		ServerDesc:                   getenv("TARMY_SERVER_DESCRIPTION", "the first universe"),
		ServerMaxUsers:               getenvInt("TARMY_SERVER_MAX_USERS", 5000),
		JWTSecret:                    getenv("TARMY_JWT_SECRET", "insecure-dev-secret-change-me"),
		JWTTTL:                       time.Duration(getenvInt("TARMY_JWT_TTL_HOURS", 168)) * time.Hour,
		BcryptCost:                   getenvInt("TARMY_BCRYPT_COST", 12),
		SchedulerTick:                time.Duration(getenvInt("TARMY_SCHEDULER_INTERVAL_SECONDS", 5)) * time.Second,
		LogLevel:                     getenv("TARMY_LOG_LEVEL", "info"),
		LogFormat:                    getenv("TARMY_LOG_FORMAT", "text"),
		DefaultUniverseName:          getenv("TARMY_DEFAULT_UNIVERSE_NAME", "Genesis"),
		DefaultUniverseGalaxies:      getenvInt("TARMY_DEFAULT_UNIVERSE_GALAXIES", 9),
		DefaultUniverseSystems:       getenvInt("TARMY_DEFAULT_UNIVERSE_SYSTEMS", 499),
		DefaultUniverseSpeedEconomy:  getenvInt("TARMY_DEFAULT_UNIVERSE_SPEED_ECONOMY", 1),
		DefaultUniverseSpeedFleet:    getenvInt("TARMY_DEFAULT_UNIVERSE_SPEED_FLEET", 1),
		DefaultUniverseSpeedResearch: getenvInt("TARMY_DEFAULT_UNIVERSE_SPEED_RESEARCH", 1),
	}
	if len(c.JWTSecret) < 16 {
		return nil, fmt.Errorf("TARMY_JWT_SECRET must be at least 16 characters")
	}
	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
