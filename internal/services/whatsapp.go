package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/school/attendance-backend/internal/config"
)

var nonDigitRegex = regexp.MustCompile(`[^\d+]`)

type WhatsAppClient struct {
	env                string
	apiURL             string
	token              string
	phoneNumberID      string
	templateName       string
	templateLang       string
	defaultCountryCode string
	httpClient         *http.Client
}

func NewWhatsAppClient(cfg *config.Config) *WhatsAppClient {
	return &WhatsAppClient{
		env:                cfg.Env,
		apiURL:             strings.TrimRight(cfg.WhatsAppAPIURL, "/"),
		token:              cfg.WhatsAppAPIToken,
		phoneNumberID:      cfg.WhatsAppPhoneNumberID,
		templateName:       cfg.WhatsAppTemplateName,
		templateLang:       cfg.WhatsAppTemplateLang,
		defaultCountryCode: cfg.DefaultCountryCode,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type metaTextParam struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type metaComponent struct {
	Type       string          `json:"type"`
	Parameters []metaTextParam `json:"parameters"`
}

type metaLanguage struct {
	Code string `json:"code"`
}

type metaTemplate struct {
	Name       string          `json:"name"`
	Language   metaLanguage    `json:"language"`
	Components []metaComponent `json:"components,omitempty"`
}

type metaSendMessagePayload struct {
	MessagingProduct string       `json:"messaging_product"`
	RecipientType    string       `json:"recipient_type"`
	To               string       `json:"to"`
	Type             string       `json:"type"`
	Template         metaTemplate `json:"template"`
}

type metaSendMessageResponse struct {
	MessagingProduct string `json:"messaging_product"`
	Contacts         []struct {
		Input string `json:"input"`
		WaID  string `json:"wa_id"`
	} `json:"contacts"`
	Messages []struct {
		ID string `json:"id"`
	} `json:"messages"`
	Error *struct {
		Message   string `json:"message"`
		Type      string `json:"type"`
		Code      int    `json:"code"`
		FBTraceID string `json:"fbtrace_id"`
	} `json:"error,omitempty"`
}

// CleanPhoneNumber normalizes guardian phone number into clean digits / E.164.
// If given a 10-digit local number, prepends defaultCountryCode if provided.
func CleanPhoneNumber(phone string, defaultCountryCode ...string) string {
	cleaned := nonDigitRegex.ReplaceAllString(phone, "")
	cleaned = strings.TrimPrefix(cleaned, "+")
	if len(cleaned) == 10 && len(defaultCountryCode) > 0 && defaultCountryCode[0] != "" {
		cleaned = strings.TrimPrefix(defaultCountryCode[0], "+") + cleaned
	}
	return cleaned
}

// SendAbsentNotification dispatches Meta Cloud API template message to guardian.
func (c *WhatsAppClient) SendAbsentNotification(
	ctx context.Context,
	recipientPhone string,
	studentName string,
	dateStr string,
	schoolName string,
) (messageID string, err error) {
	phone := CleanPhoneNumber(recipientPhone, c.defaultCountryCode)
	if phone == "" {
		return "", fmt.Errorf("recipient phone number is empty or invalid after formatting")
	}

	if c.token == "" || c.phoneNumberID == "" {
		if c.env == "production" {
			return "", fmt.Errorf("whatsapp production credentials not configured: WHATSAPP_API_TOKEN or WHATSAPP_PHONE_NUMBER_ID is empty")
		}
		mockID := fmt.Sprintf("mock_msg_%d_%s", time.Now().UnixNano(), phone)
		log.Printf("[WhatsApp MOCK] Token/PhoneID not set. Mocking send to %s for student %s on %s (school: %s)", phone, studentName, dateStr, schoolName)
		return mockID, nil
	}

	var components []metaComponent
	if c.templateName != "hello_world" {
		var params []metaTextParam
		if c.templateName == "jaspers_market_order_confirmation_v1" {
			params = []metaTextParam{
				{Type: "text", Text: studentName},
				{Type: "text", Text: "1"},      // Quantity (e.g. 1 day of absence)
				{Type: "text", Text: "$0.00"},  // Price (dummy price)
			}
		} else {
			params = []metaTextParam{
				{Type: "text", Text: studentName},
				{Type: "text", Text: dateStr},
				{Type: "text", Text: schoolName},
			}
		}
		components = []metaComponent{
			{
				Type:       "body",
				Parameters: params,
			},
		}
	}

	payload := metaSendMessagePayload{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               phone,
		Type:             "template",
		Template: metaTemplate{
			Name: c.templateName,
			Language: metaLanguage{
				Code: c.templateLang,
			},
			Components: components,
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal json payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/%s/messages", c.apiURL, c.phoneNumberID)
	log.Printf("[WhatsApp debug] Sending Request to URL: %s, Payload: %s", endpoint, string(bodyBytes))

	maxAttempts := 3
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			log.Printf("[WhatsApp debug] Attempt %d HTTP request error: %v", attempt, err)
			lastErr = fmt.Errorf("http request failed: %w", err)
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt*500) * time.Millisecond)
				continue
			}
			return "", lastErr
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		log.Printf("[WhatsApp debug] Attempt %d HTTP response status: %d, Body: %s", attempt, resp.StatusCode, string(respBody))
		if err != nil {
			lastErr = fmt.Errorf("read response body: %w", err)
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt*500) * time.Millisecond)
				continue
			}
			return "", lastErr
		}

		// Loud critical alerting on auth failure (401 / 403)
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			log.Printf("[CRITICAL ALERT] Meta WhatsApp API auth failure (HTTP %d). Token is invalid or expired! Response: %s", resp.StatusCode, string(respBody))
			return "", fmt.Errorf("meta whatsapp API authentication failed (HTTP %d): token invalid or expired", resp.StatusCode)
		}

		// Backoff retry on Rate Limit (429) or Meta server error (5xx)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("meta API temporary failure (HTTP %d): %s", resp.StatusCode, string(respBody))
			if attempt < maxAttempts {
				log.Printf("[WhatsApp WARN] Meta API returned HTTP %d on attempt %d/%d; retrying in %dms...", resp.StatusCode, attempt, maxAttempts, attempt*500)
				time.Sleep(time.Duration(attempt*500) * time.Millisecond)
				continue
			}
			return "", lastErr
		}

		var metaResp metaSendMessageResponse
		if err := json.Unmarshal(respBody, &metaResp); err != nil {
			return "", fmt.Errorf("unmarshal meta response (status %d, body %q): %w", resp.StatusCode, string(respBody), err)
		}

		if metaResp.Error != nil {
			return "", fmt.Errorf("meta api error (code %d): %s", metaResp.Error.Code, metaResp.Error.Message)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("meta api HTTP status %d: %s", resp.StatusCode, string(respBody))
		}

		if len(metaResp.Messages) > 0 {
			return metaResp.Messages[0].ID, nil
		}

		return fmt.Sprintf("sent_status_%d", resp.StatusCode), nil
	}

	return "", lastErr
}

type AbsentStudentNotificationInfo struct {
	SchoolID      uuid.UUID
	SessionID     uuid.UUID
	StudentID     uuid.UUID
	StudentName   string
	GuardianPhone string
	SessionDate   time.Time
	SchoolName    string
}
