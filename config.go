package main

import (
	"os"
)

type Config struct {
	Port              string
	BotToken          string
	DatabasePath      string
	MigrationsPath    string
	SessionSecret     string
	MaxBookingsPerDay int
}

func LoadConfig() *Config {
	return &Config{
		Port:              getEnvOrDefault("PORT", "8080"),
		BotToken:          getEnvOrDefault("TELEGRAM_BOT_TOKEN", ""),
		DatabasePath:      "mvp_chatbot.db",
		MigrationsPath:    "migrations_v2.sql",
		SessionSecret:     "secret",
		MaxBookingsPerDay: 8,
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
