package tests

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/school/attendance-backend/internal/config"
	"github.com/school/attendance-backend/internal/models"
	"github.com/school/attendance-backend/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ensureWhatsAppSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, _ = pool.Exec(ctx, "ALTER TABLE schools ADD COLUMN IF NOT EXISTS is_whatsapp_enabled BOOLEAN NOT NULL DEFAULT FALSE")
	_, _ = pool.Exec(ctx, "ALTER TABLE schools ADD COLUMN IF NOT EXISTS whatsapp_monthly_limit INT NOT NULL DEFAULT 1000")
	_, _ = pool.Exec(ctx, "ALTER TABLE attendance_sessions ADD COLUMN IF NOT EXISTS is_finalized BOOLEAN NOT NULL DEFAULT FALSE")
	_, _ = pool.Exec(ctx, "ALTER TABLE attendance_sessions ADD COLUMN IF NOT EXISTS finalized_at TIMESTAMPTZ")
	_, _ = pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS whatsapp_logs (
		id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
		school_id       UUID        NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
		session_id      UUID        REFERENCES attendance_sessions(id) ON DELETE SET NULL,
		student_id      UUID        REFERENCES students(id) ON DELETE SET NULL,
		recipient_phone VARCHAR(50) NOT NULL,
		status          VARCHAR(30) NOT NULL,
		meta_message_id VARCHAR(255),
		error_message   TEXT,
		created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
}

func TestProductionConfig_FailFastValidation(t *testing.T) {
	os.Setenv("ENV", "production")
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars-long")
	os.Setenv("WHATSAPP_API_TOKEN", "")
	os.Setenv("WHATSAPP_PHONE_NUMBER_ID", "")
	defer func() {
		os.Setenv("ENV", "development")
	}()

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WHATSAPP_API_TOKEN is required when ENV=production")
}

func TestWhatsAppWebhook_GetVerification(t *testing.T) {
	e := newTestEnv(t)
	ensureWhatsAppSchema(t, e.pool)

	// Set webhook verify token
	e.cfg.WhatsAppWebhookVerifyToken = "my_secret_verify_token"

	// 1. Valid verify token
	url := fmt.Sprintf("%s/webhook/whatsapp?hub.mode=subscribe&hub.verify_token=my_secret_verify_token&hub.challenge=123456789", e.srv.URL)
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body string
	buf := make([]byte, 100)
	n, _ := resp.Body.Read(buf)
	body = string(buf[:n])
	assert.Equal(t, "123456789", body)

	// 2. Invalid verify token -> 403 Forbidden
	invalidURL := fmt.Sprintf("%s/webhook/whatsapp?hub.mode=subscribe&hub.verify_token=wrong_token&hub.challenge=123456789", e.srv.URL)
	resp2, err := http.Get(invalidURL)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp2.StatusCode)
}

func TestWhatsAppWebhook_PostStatusUpdate(t *testing.T) {
	e := newTestEnv(t)
	ensureWhatsAppSchema(t, e.pool)
	ctx := context.Background()

	// Ensure DB table whatsapp_logs has a test record
	metaMsgID := fmt.Sprintf("wamid_test_%d", time.Now().UnixNano())
	schoolCode, _, _ := seedTestSchool(t, e.pool, fmt.Sprintf("wh-%d", time.Now().UnixNano()))
	var schoolID string
	err := e.pool.QueryRow(ctx, "SELECT id FROM schools WHERE code = $1", schoolCode).Scan(&schoolID)
	require.NoError(t, err)

	whatsappRepo := repository.NewWhatsAppRepo(repository.NewDB(e.pool))
	schoolUUID, _ := uuid.Parse(schoolID)

	// Insert initial 'sent' log record
	logEntry := &models.WhatsAppLog{
		SchoolID:       schoolUUID,
		RecipientPhone: "919876543210",
		Status:         "sent",
		MetaMessageID:  &metaMsgID,
	}
	require.NoError(t, whatsappRepo.LogWhatsAppMessage(ctx, logEntry))

	// Configure app secret
	secret := "test_meta_app_secret"
	e.cfg.WhatsAppAppSecret = secret

	// Construct Meta webhook payload
	payloadJSON := fmt.Sprintf(`{
		"object": "whatsapp_business_account",
		"entry": [{
			"id": "waba_id_123",
			"changes": [{
				"field": "messages",
				"value": {
					"messaging_product": "whatsapp",
					"metadata": {"display_phone_number": "1555001234", "phone_number_id": "123456"},
					"statuses": [{
						"id": "%s",
						"recipient_id": "919876543210",
						"status": "delivered",
						"timestamp": "1600000000"
					}]
				}
			}]
		}]
	}`, metaMsgID)

	// Compute HMAC SHA256 signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payloadJSON))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequest(http.MethodPost, e.srv.URL+"/webhook/whatsapp", bytes.NewReader([]byte(payloadJSON)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Give async webhook handler goroutine a moment to update status
	time.Sleep(200 * time.Millisecond)

	// Query DB for updated status
	var updatedStatus string
	err = e.pool.QueryRow(ctx, "SELECT status FROM whatsapp_logs WHERE meta_message_id = $1", metaMsgID).Scan(&updatedStatus)
	require.NoError(t, err)
	assert.Equal(t, "delivered", updatedStatus)
}
