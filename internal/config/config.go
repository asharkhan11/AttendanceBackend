package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                 string
	Env                  string
	DatabaseURL          string
	MigrationsURL        string
	JWTSecret            string
	JWTAccessTTL         time.Duration
	JWTRefreshTTL        time.Duration
	Argon2MemoryKB       uint32
	Argon2Iterations     uint32
	Argon2Parallelism    uint8
	CORSAllowedOrigins   []string
	RateLimitLoginPerMin int

	// Meta WhatsApp Cloud API Configuration
	EnableWhatsApp             bool
	WhatsAppAPIToken           string
	WhatsAppPhoneNumberID      string
	WhatsAppTemplateName       string
	WhatsAppTemplateLang       string
	WhatsAppAPIURL             string
	WhatsAppAppID              string
	WhatsAppAppSecret          string
	WhatsAppWABAID             string
	WhatsAppWebhookVerifyToken  string
	DefaultCountryCode         string

	// Database pool configuration
	DBMaxConns        int32
	DBMinConns        int32
	DBMaxConnIdleTime time.Duration
	DBMaxConnLifetime time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{}

	cfg.Port = getEnv("PORT", "8080")
	cfg.Env = getEnv("ENV", "development")

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	cfg.JWTSecret = os.Getenv("JWT_SECRET")
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	var err error
	cfg.JWTAccessTTL, err = parseDuration("JWT_ACCESS_TTL", "15m")
	if err != nil {
		return nil, err
	}
	cfg.JWTRefreshTTL, err = parseDuration("JWT_REFRESH_TTL", "720h")
	if err != nil {
		return nil, err
	}

	memKB, err := parseUint32("ARGON2_MEMORY_KB", 65536)
	if err != nil {
		return nil, err
	}
	cfg.Argon2MemoryKB = memKB

	iters, err := parseUint32("ARGON2_ITERATIONS", 3)
	if err != nil {
		return nil, err
	}
	cfg.Argon2Iterations = iters

	para, err := parseUint8("ARGON2_PARALLELISM", 2)
	if err != nil {
		return nil, err
	}
	cfg.Argon2Parallelism = para

	originsStr := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173")
	cfg.CORSAllowedOrigins = splitTrim(originsStr, ",")

	rl, err := parseInt("RATE_LIMIT_LOGIN_PER_MIN", 10)
	if err != nil {
		return nil, err
	}
	cfg.RateLimitLoginPerMin = rl

	cfg.EnableWhatsApp = strings.EqualFold(getEnv("ENABLE_WHATSAPP", "false"), "true")
	cfg.WhatsAppAPIToken = getEnv("WHATSAPP_API_TOKEN", os.Getenv("WHATSAPP_PERMANENT_TOKEN"))
	cfg.WhatsAppPhoneNumberID = getEnv("WHATSAPP_PHONE_NUMBER_ID", "")
	cfg.WhatsAppTemplateName = getEnv("WHATSAPP_TEMPLATE_NAME", "absent_student_notification")
	cfg.WhatsAppTemplateLang = getEnv("WHATSAPP_TEMPLATE_LANG", "en")
	cfg.WhatsAppAPIURL = getEnv("WHATSAPP_API_URL", "https://graph.facebook.com/v20.0")
	cfg.WhatsAppAppID = getEnv("WHATSAPP_APP_ID", "")
	cfg.WhatsAppAppSecret = getEnv("WHATSAPP_APP_SECRET", "")
	cfg.WhatsAppWABAID = getEnv("WHATSAPP_WABA_ID", os.Getenv("WHATSAPP_BUSINESS_ACCOUNT_ID"))
	cfg.WhatsAppWebhookVerifyToken = getEnv("WHATSAPP_WEBHOOK_VERIFY_TOKEN", "")
	cfg.DefaultCountryCode = getEnv("DEFAULT_COUNTRY_CODE", "91")

	if cfg.Env == "production" && cfg.EnableWhatsApp {
		if cfg.WhatsAppAPIToken == "" {
			return nil, fmt.Errorf("WHATSAPP_API_TOKEN is required when ENV=production (or set WHATSAPP_PERMANENT_TOKEN)")
		}
		if cfg.WhatsAppPhoneNumberID == "" {
			return nil, fmt.Errorf("WHATSAPP_PHONE_NUMBER_ID is required when ENV=production")
		}
		if cfg.WhatsAppWebhookVerifyToken == "" {
			return nil, fmt.Errorf("WHATSAPP_WEBHOOK_VERIFY_TOKEN is required when ENV=production")
		}
		if cfg.WhatsAppAppSecret == "" {
			return nil, fmt.Errorf("WHATSAPP_APP_SECRET is required when ENV=production")
		}
	}

	// Build migrations URL: same DB but we need file-based migration path
	cfg.MigrationsURL = "file://internal/db/migrations"

	// Database pool configuration
	maxConns, err := parseInt("DB_MAX_CONNS", 20)
	if err != nil {
		return nil, err
	}
	cfg.DBMaxConns = int32(maxConns)

	minConns, err := parseInt("DB_MIN_CONNS", 2)
	if err != nil {
		return nil, err
	}
	cfg.DBMinConns = int32(minConns)

	cfg.DBMaxConnIdleTime, err = parseDuration("DB_MAX_CONN_IDLE_TIME", "30m")
	if err != nil {
		return nil, err
	}
	cfg.DBMaxConnLifetime, err = parseDuration("DB_MAX_CONN_LIFETIME", "1h")
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseDuration(key, def string) (time.Duration, error) {
	s := getEnv(key, def)
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid %s=%q: %w", key, s, err)
	}
	return d, nil
}

func parseUint32(key string, def uint32) (uint32, error) {
	s := getEnv(key, strconv.Itoa(int(def)))
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid %s=%q: %w", key, s, err)
	}
	return uint32(v), nil
}

func parseUint8(key string, def uint8) (uint8, error) {
	s := getEnv(key, strconv.Itoa(int(def)))
	v, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid %s=%q: %w", key, s, err)
	}
	return uint8(v), nil
}

func parseInt(key string, def int) (int, error) {
	s := getEnv(key, strconv.Itoa(def))
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid %s=%q: %w", key, s, err)
	}
	return v, nil
}

func splitTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}
