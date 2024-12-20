package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Domain string
	DB     string
	Smtp   SmtpConfig
	Stripe StripeConfig
}

type SmtpConfig struct {
	Host     string
	Address  string
	Username string
	Password string
}

type StripeConfig struct {
	Key      string
	Endpoint string
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	cfg := &Config{
		Domain: os.Getenv("DOMAIN_URL"),
		DB:     os.Getenv("DB_NAME"),
		Smtp: SmtpConfig{
			Host:     os.Getenv("SMTP_HOST"),
			Address:  os.Getenv("SMTP_ADDRESS"),
			Username: os.Getenv("SMTP_USERNAME"),
			Password: os.Getenv("SMTP_PASSWORD"),
		},
		Stripe: StripeConfig{
			Key:      os.Getenv("STRIPE_KEY"),
			Endpoint: os.Getenv("STRIPE_ENDPOINT_SECRET"),
		},
	}

	return cfg
}
