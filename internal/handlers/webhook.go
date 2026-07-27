package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/school/attendance-backend/internal/config"
	"github.com/school/attendance-backend/internal/repository"
)

type WhatsAppWebhookHandler struct {
	cfg          *config.Config
	whatsappRepo *repository.WhatsAppRepo
}

func NewWhatsAppWebhookHandler(cfg *config.Config, whatsappRepo *repository.WhatsAppRepo) *WhatsAppWebhookHandler {
	return &WhatsAppWebhookHandler{
		cfg:          cfg,
		whatsappRepo: whatsappRepo,
	}
}

// VerifyWebhook handles Meta GET verification requests (hub.mode, hub.verify_token, hub.challenge).
func (h *WhatsAppWebhookHandler) VerifyWebhook(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	mode := q.Get("hub.mode")
	token := q.Get("hub.verify_token")
	challenge := q.Get("hub.challenge")

	if mode == "subscribe" && h.cfg.WhatsAppWebhookVerifyToken != "" && token == h.cfg.WhatsAppWebhookVerifyToken {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(challenge))
		return
	}

	http.Error(w, "Forbidden: verification token mismatch", http.StatusForbidden)
}

// Meta Webhook Payload Data Structures
type metaWebhookPayload struct {
	Object string             `json:"object"`
	Entry  []metaWebhookEntry `json:"entry"`
}

type metaWebhookEntry struct {
	ID      string              `json:"id"`
	Changes []metaWebhookChange `json:"changes"`
}

type metaWebhookChange struct {
	Value metaWebhookValue `json:"value"`
	Field string           `json:"field"`
}

type metaWebhookValue struct {
	MessagingProduct string               `json:"messaging_product"`
	Metadata         metaWebhookMetadata  `json:"metadata"`
	Statuses         []metaWebhookStatus  `json:"statuses"`
	Messages         []metaWebhookMessage `json:"messages"`
}

type metaWebhookMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type metaWebhookStatus struct {
	ID           string             `json:"id"`
	RecipientID  string             `json:"recipient_id"`
	Status       string             `json:"status"` // sent, delivered, read, failed
	Timestamp    string             `json:"timestamp"`
	Conversation map[string]any    `json:"conversation"`
	Pricing      map[string]any    `json:"pricing"`
	Errors       []metaWebhookError `json:"errors"`
}

type metaWebhookError struct {
	Code  int    `json:"code"`
	Title string `json:"title"`
}

type metaWebhookMessage struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
}

// HandleWebhook handles Meta POST notifications (status updates and incoming messages).
func (h *WhatsAppWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Validate HMAC SHA256 signature if App Secret is configured
	if h.cfg.WhatsAppAppSecret != "" {
		signatureHeader := r.Header.Get("X-Hub-Signature-256")
		if !isValidHMACSignature(body, signatureHeader, h.cfg.WhatsAppAppSecret) {
			log.Printf("[WhatsApp Webhook WARN] Invalid X-Hub-Signature-256 signature")
			http.Error(w, "Unauthorized: invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Immediately respond 200 OK to Meta
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("EVENT_RECEIVED"))

	// Asynchronously process status updates without delaying HTTP response
	go h.processWebhookPayload(body)
}

func (h *WhatsAppWebhookHandler) processWebhookPayload(body []byte) {
	var payload metaWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[WhatsApp Webhook ERROR] Failed to parse webhook JSON: %v. Raw body: %s", err, string(body))
		return
	}

	if payload.Object != "whatsapp_business_account" {
		log.Printf("[WhatsApp Webhook DEBUG] Unexpected object %q. Raw body: %s", payload.Object, string(body))
		return
	}

	ctx := rContext()
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}

			// Process delivery status updates (sent, delivered, read, failed)
			for _, st := range change.Value.Statuses {
				statusStr := st.Status
				var errText *string
				if len(st.Errors) > 0 {
					msg := fmt.Sprintf("Meta error %d: %s", st.Errors[0].Code, st.Errors[0].Title)
					errText = &msg
				}

				if err := h.whatsappRepo.UpdateWhatsAppLogStatusByMetaID(ctx, st.ID, statusStr, errText); err != nil {
					log.Printf("[WhatsApp Webhook ERROR] Failed to update status for message %s: %v", st.ID, err)
				} else {
					log.Printf("[WhatsApp Webhook] Message %s status updated to '%s'", st.ID, statusStr)
				}
			}

			// Handle incoming messages (send-only design: acknowledge & log, do not process further)
			for _, msg := range change.Value.Messages {
				log.Printf("[WhatsApp Webhook INFO] Received incoming guardian message from %s (ID: %s, type: %s). Ignored per send-only policy.", msg.From, msg.ID, msg.Type)
			}
		}
	}
}

func isValidHMACSignature(body []byte, signatureHeader string, secret string) bool {
	if signatureHeader == "" {
		return false
	}
	parts := strings.SplitN(signatureHeader, "=", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		return false
	}
	expectedHash := parts[1]

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	actualHash := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(actualHash), []byte(expectedHash))
}

func rContext() context.Context {
	return context.Background()
}
