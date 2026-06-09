package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppPort           string
	DBHost            string
	DBPort            string
	DBUser            string
	DBPassword        string
	DBName            string
	DBSSLMode         string
	JWTSecret         string
	HMACSecret        string
	PGPPassphrase     string
	CreditMargin      float64
	SchedulerInterval time.Duration
	SMTPHost          string
	SMTPPort          int
	SMTPUser          string
	SMTPPassword      string
	SMTPFrom          string
	LogLevel          string
}

func Load() Config {
	margin, _ := strconv.ParseFloat(getenv("CREDIT_MARGIN", "5"), 64)

	smtpPort, _ := strconv.Atoi(getenv("SMTP_PORT", "1025"))

	interval, err := time.ParseDuration(getenv("SCHEDULER_INTERVAL", "12h"))
	if err != nil || interval <= 0 {
		interval = 12 * time.Hour
	}

	return Config{
		AppPort:           getenv("APP_PORT", "8080"),
		DBHost:            getenv("DB_HOST", "localhost"),
		DBPort:            getenv("DB_PORT", "5432"),
		DBUser:            getenv("DB_USER", "bank"),
		DBPassword:        getenv("DB_PASSWORD", "bank"),
		DBName:            getenv("DB_NAME", "bank"),
		DBSSLMode:         getenv("DB_SSLMODE", "disable"),
		JWTSecret:         getenv("JWT_SECRET", "dev_jwt_secret_change_me"),
		HMACSecret:        getenv("HMAC_SECRET", "dev_hmac_secret_change_me"),
		PGPPassphrase:     getenv("PGP_PASSPHRASE", "dev_pgp_passphrase_change_me"),
		CreditMargin:      margin,
		SchedulerInterval: interval,
		SMTPHost:          getenv("SMTP_HOST", ""),
		SMTPPort:          smtpPort,
		SMTPUser:          getenv("SMTP_USER", ""),
		SMTPPassword:      getenv("SMTP_PASSWORD", ""),
		SMTPFrom:          getenv("SMTP_FROM", "noreply@bank.local"),
		LogLevel:          getenv("LOG_LEVEL", "debug"),
	}
}

func (c Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
