package config

import (
	"log"
	"os"
)

type Config struct {
	TelegramBotToken string
	OpenAIAPIKey     string
	OpenAIImageModel string
	DatabaseURL      string
	AppEnv           string
	HTTPAddr         string
}

func Load() *Config {
	return &Config{
		TelegramBotToken: requireEnv("TELEGRAM_BOT_TOKEN"),
		OpenAIAPIKey:     requireEnv("OPENAI_API_KEY"),
		OpenAIImageModel: getEnv("OPENAI_IMAGE_MODEL", "gpt-image-1"),
		DatabaseURL:      requireEnv("DATABASE_URL"),
		AppEnv:           getEnv("APP_ENV", "development"),
		HTTPAddr:         getEnv("HTTP_ADDR", ":8080"),
	}
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}

	return v
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return defaultValue
}
