package config_test

import (
	"os"
	"testing"

	"github.com/school/attendance-backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_ProductionValidation(t *testing.T) {
	// Backup original env vars
	origEnv := os.Getenv("ENV")
	origDB := os.Getenv("DATABASE_URL")
	origJWT := os.Getenv("JWT_SECRET")
	origToken := os.Getenv("WHATSAPP_API_TOKEN")
	origPhone := os.Getenv("WHATSAPP_PHONE_NUMBER_ID")
	origVerify := os.Getenv("WHATSAPP_WEBHOOK_VERIFY_TOKEN")
	origSecret := os.Getenv("WHATSAPP_APP_SECRET")

	defer func() {
		os.Setenv("ENV", origEnv)
		os.Setenv("DATABASE_URL", origDB)
		os.Setenv("JWT_SECRET", origJWT)
		os.Setenv("WHATSAPP_API_TOKEN", origToken)
		os.Setenv("WHATSAPP_PHONE_NUMBER_ID", origPhone)
		os.Setenv("WHATSAPP_WEBHOOK_VERIFY_TOKEN", origVerify)
		os.Setenv("WHATSAPP_APP_SECRET", origSecret)
	}()

	os.Setenv("ENV", "production")
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	os.Setenv("JWT_SECRET", "this-is-a-very-long-test-jwt-secret-key-32-bytes")

	// Missing WHATSAPP_API_TOKEN
	os.Setenv("WHATSAPP_API_TOKEN", "")
	os.Setenv("WHATSAPP_PHONE_NUMBER_ID", "123456789")
	os.Setenv("WHATSAPP_WEBHOOK_VERIFY_TOKEN", "vtoken")
	os.Setenv("WHATSAPP_APP_SECRET", "appsecret")
	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WHATSAPP_API_TOKEN")

	// Missing WHATSAPP_PHONE_NUMBER_ID
	os.Setenv("WHATSAPP_API_TOKEN", "my_token")
	os.Setenv("WHATSAPP_PHONE_NUMBER_ID", "")
	_, err = config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WHATSAPP_PHONE_NUMBER_ID")

	// Missing WHATSAPP_WEBHOOK_VERIFY_TOKEN
	os.Setenv("WHATSAPP_PHONE_NUMBER_ID", "123456789")
	os.Setenv("WHATSAPP_WEBHOOK_VERIFY_TOKEN", "")
	_, err = config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WHATSAPP_WEBHOOK_VERIFY_TOKEN")

	// Missing WHATSAPP_APP_SECRET
	os.Setenv("WHATSAPP_WEBHOOK_VERIFY_TOKEN", "vtoken")
	os.Setenv("WHATSAPP_APP_SECRET", "")
	_, err = config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WHATSAPP_APP_SECRET")

	// All present
	os.Setenv("WHATSAPP_APP_SECRET", "appsecret")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "my_token", cfg.WhatsAppAPIToken)
	assert.Equal(t, "123456789", cfg.WhatsAppPhoneNumberID)
	assert.Equal(t, "vtoken", cfg.WhatsAppWebhookVerifyToken)
	assert.Equal(t, "appsecret", cfg.WhatsAppAppSecret)
}
