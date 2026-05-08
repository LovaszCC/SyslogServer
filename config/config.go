package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	SyslogPort    string
	ProxyProtocol bool
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBName        string
	DBSSLMode     string
}

func Load() *Config {
	return &Config{
		SyslogPort:    getEnv("SYSLOG_PORT", "514"),
		ProxyProtocol: getEnvBool("PROXY_PROTOCOL", false),
		DBHost:        getEnv("DB_HOST", "localhost"),
		DBPort:        getEnv("DB_PORT", "5432"),
		DBUser:        getEnv("DB_USER", "syslog"),
		DBPassword:    getEnv("DB_PASSWORD", "syslog"),
		DBName:        getEnv("DB_NAME", "syslog"),
		DBSSLMode:     getEnv("DB_SSLMODE", "disable"),
	}
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
