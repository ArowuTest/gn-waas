package service

// SMSService sends SMS notifications via Hubtel (primary) or Arkesel (fallback).
//
// GHANA CONTEXT — Why SMS over email:
//   Ghana's primary communication channel is SMS. Field officers in rural areas
//   may not have smartphones or reliable internet, but almost everyone has a
//   basic phone that receives SMS. Email is rarely checked.
//
//   SMS Gateways used in Ghana:
//   1. Hubtel (https://developers.hubtel.com) — most widely used, GHS 0.04/SMS
//   2. Arkesel (https://arkesel.com) — good coverage, GHS 0.035/SMS
//   3. Wigal (https://wigal.com.gh) — government-preferred gateway
//
//   This service uses Hubtel as primary with automatic fallback to Arkesel.
//   Both APIs are REST-based with similar request/response structures.
//
// SMS Templates:
//   All templates support {variable} substitution.
//   Templates are stored in i18n_strings table for multi-language support.
//   Default language is English; Twi (Akan) available for customer-facing SMS.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// SMSService sends SMS via Hubtel or Arkesel
type SMSService struct {
	hubtelClientID     string
	hubtelClientSecret string
	hubtelSenderID     string // e.g. "GNWAAS"
	arkeselAPIKey      string
	arkeselSenderID    string
	client             *http.Client
	db                 *pgxpool.Pool
	logger             *zap.Logger
	sandboxMode        bool
}

// SMSConfig holds SMS gateway configuration
type SMSConfig struct {
	HubtelClientID     string
	HubtelClientSecret string
	HubtelSenderID     string
	ArkeselAPIKey      string
	ArkeselSenderID    string
	SandboxMode        bool
}

func NewSMSService(cfg SMSConfig, db *pgxpool.Pool, logger *zap.Logger) *SMSService {
	return &SMSService{
		hubtelClientID:     cfg.HubtelClientID,
		hubtelClientSecret: cfg.HubtelClientSecret,
		hubtelSenderID:     cfg.HubtelSenderID,
		arkeselAPIKey:      cfg.ArkeselAPIKey,
		arkeselSenderID:    cfg.ArkeselSenderID,
		client:             &http.Client{Timeout: 15 * time.Second},
		db:                 db,
		logger:             logger,
		sandboxMode:        cfg.SandboxMode,
	}
}

// NewSMSServiceFromEnv creates an SMSService from environment variables
func NewSMSServiceFromEnv(db *pgxpool.Pool, logger *zap.Logger) *SMSService {
	return NewSMSService(SMSConfig{
		HubtelClientID:     os.Getenv("HUBTEL_CLIENT_ID"),
		HubtelClientSecret: os.Getenv("HUBTEL_CLIENT_SECRET"),
		HubtelSenderID:     getEnvOrDefault("HUBTEL_SENDER_ID", "GNWAAS"),
		ArkeselAPIKey:      os.Getenv("ARKESEL_API_KEY"),
		ArkeselSenderID:    getEnvOrDefault("ARKESEL_SENDER_ID", "GNWAAS"),
		SandboxMode:        os.Getenv("SMS_SANDBOX_MODE") == "true",
	}, db, logger)
}

// ── SEND FUNCTIONS ────────────────────────────────────────────────────────────

// SendJobAssigned notifies a field officer of a new job assignment
func (s *SMSService) SendJobAssigned(
	ctx context.Context,
	officerPhone string,
	officerName string,
	jobRef string,
	districtName string,
	dueDate string,
) error {
	msg := fmt.Sprintf(
		"GN-WAAS: New field job assigned. Ref: %s, District: %s, Due: %s. Login to app for details.",
		jobRef, districtName, dueDate,
	)
	return s.Send(ctx, &SMSRequest{
		RecipientPhone:   officerPhone,
		RecipientName:    officerName,
		Message:          msg,
		Template:         "JOB_ASSIGNED",
		RelatedEntityType: "field_job",
	})
}

// SendAuditComplete notifies a customer that their audit is complete
func (s *SMSService) SendAuditComplete(
	ctx context.Context,
	customerPhone string,
	customerName string,
	accountNumber string,
	auditRef string,
	graStatus string,
) error {
	var msg string
	if graStatus == "SIGNED" {
		msg = fmt.Sprintf(
			"GN-WAAS: Audit complete for account %s. Ref: %s. GRA-signed receipt issued. Queries: 0800-GNWAAS",
			accountNumber, auditRef,
		)
	} else {
		msg = fmt.Sprintf(
			"GN-WAAS: Audit complete for account %s. Ref: %s (provisional). GRA signing pending. Queries: 0800-GNWAAS",
			accountNumber, auditRef,
		)
	}
	return s.Send(ctx, &SMSRequest{
		RecipientPhone:   customerPhone,
		RecipientName:    customerName,
		Message:          msg,
		Template:         "AUDIT_COMPLETE",
		RelatedEntityType: "audit_event",
	})
}

// SendAnomalyAlert notifies a supervisor of anomalies in their district
func (s *SMSService) SendAnomalyAlert(
	ctx context.Context,
	supervisorPhone string,
	supervisorName string,
	districtName string,
	anomalyCount int,
	totalVarianceGHS float64,
) error {
	msg := fmt.Sprintf(
		"GN-WAAS ALERT: %d anomalies detected in %s. Est. revenue gap: GH₵%.2f. Login to review.",
		anomalyCount, districtName, totalVarianceGHS,
	)
	return s.Send(ctx, &SMSRequest{
		RecipientPhone:   supervisorPhone,
		RecipientName:    supervisorName,
		Message:          msg,
		Template:         "ANOMALY_ALERT",
		RelatedEntityType: "anomaly",
	})
}

// SendGRASigned notifies an audit manager that GRA signing is confirmed
func (s *SMSService) SendGRASigned(
	ctx context.Context,
	managerPhone string,
	managerName string,
	auditRef string,
	sdcID string,
) error {
	msg := fmt.Sprintf(
		"GN-WAAS: GRA signing confirmed for audit %s. SDC-ID: %s. Evidence package sealed.",
		auditRef, sdcID,
	)
	return s.Send(ctx, &SMSRequest{
		RecipientPhone:   managerPhone,
		RecipientName:    managerName,
		Message:          msg,
		Template:         "GRA_SIGNED",
		RelatedEntityType: "audit_event",
	})
}

// SendWhistleblowerReceipt sends a tip reference to a whistleblower
func (s *SMSService) SendWhistleblowerReceipt(
	ctx context.Context,
	phone string,
	tipRef string,
) error {
	msg := fmt.Sprintf(
		"GN-WAAS: Your tip has been received. Reference: %s. We will investigate and contact you if needed. Thank you.",
		tipRef,
	)
	return s.Send(ctx, &SMSRequest{
		RecipientPhone:   phone,
		Message:          msg,
		Template:         "CUSTOM",
		RelatedEntityType: "whistleblower_tip",
	})
}

// ── CORE SEND ─────────────────────────────────────────────────────────────────

// SMSRequest holds the parameters for sending an SMS
type SMSRequest struct {
	RecipientPhone    string
	RecipientName     string
	RecipientUserID   *uuid.UUID
	Message           string
	Template          string
	RelatedEntityType string
	RelatedEntityID   *uuid.UUID
	DistrictID        *uuid.UUID
}

// Send sends an SMS and records it in the database
func (s *SMSService) Send(ctx context.Context, req *SMSRequest) error {
	// Normalise phone
	phone := normalisePhone(req.RecipientPhone)
	if phone == "" {
		return fmt.Errorf("invalid phone number: %s", req.RecipientPhone)
	}

	// Record in DB first (QUEUED)
	var smsID uuid.UUID
	err := s.db.QueryRow(ctx, `
		INSERT INTO sms_notifications (
			recipient_phone, recipient_name, recipient_user_id,
			template, message_body, provider, status,
			related_entity_type, related_entity_id, district_id
		) VALUES ($1,$2,$3,$4::sms_template,$5,'HUBTEL','QUEUED',$6,$7,$8)
		RETURNING id
	`,
		phone, req.RecipientName, req.RecipientUserID,
		req.Template, req.Message,
		req.RelatedEntityType, req.RelatedEntityID, req.DistrictID,
	).Scan(&smsID)
	if err != nil {
		s.logger.Error("Failed to record SMS in DB", zap.Error(err))
		// Continue anyway — don't fail the business operation for SMS logging
	}

	if s.sandboxMode {
		s.logger.Info("SMS SANDBOX: would send",
			zap.String("to", phone),
			zap.String("message", req.Message),
		)
		s.updateSMSStatus(ctx, smsID, "SENT", "", nil)
		return nil
	}

	// Try Hubtel first
	providerMsgID, err := s.sendViaHubtel(ctx, phone, req.Message)
	if err != nil {
		s.logger.Warn("Hubtel SMS failed, trying Arkesel",
			zap.String("phone", phone),
			zap.Error(err),
		)
		// Fallback to Arkesel
		providerMsgID, err = s.sendViaArkesel(ctx, phone, req.Message)
		if err != nil {
			s.logger.Error("Both SMS providers failed",
				zap.String("phone", phone),
				zap.Error(err),
			)
			s.updateSMSStatus(ctx, smsID, "FAILED", "", err)
			return fmt.Errorf("SMS delivery failed via both Hubtel and Arkesel: %w", err)
		}
		// Update provider to Arkesel
		s.db.Exec(ctx,
			`UPDATE sms_notifications SET provider = 'ARKESEL' WHERE id = $1`, smsID,
		)
	}

	s.updateSMSStatus(ctx, smsID, "SENT", providerMsgID, nil)
	s.logger.Info("SMS sent",
		zap.String("to", phone),
		zap.String("provider_msg_id", providerMsgID),
	)
	return nil
}

// ── HUBTEL API ────────────────────────────────────────────────────────────────

type hubtelSendRequest struct {
	From    string `json:"From"`
	To      string `json:"To"`
	Content string `json:"Content"`
}

type hubtelSendResponse struct {
	Status  int    `json:"Status"`
	Message string `json:"Message"`
	Data    struct {
		MessageID string `json:"MessageId"`
		Rate      float64 `json:"Rate"`
	} `json:"Data"`
}

func (s *SMSService) sendViaHubtel(ctx context.Context, phone, message string) (string, error) {
	if s.hubtelClientID == "" {
		return "", fmt.Errorf("Hubtel credentials not configured")
	}

	payload := hubtelSendRequest{
		From:    s.hubtelSenderID,
		To:      phone,
		Content: message,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://smsc.hubtel.com/v1/messages/send", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	// Hubtel uses Basic Auth
	auth := base64.StdEncoding.EncodeToString(
		[]byte(s.hubtelClientID + ":" + s.hubtelClientSecret),
	)
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Hubtel request failed: %w", err)
	}
	defer resp.Body.Close()

	var result hubtelSendResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse Hubtel response: %w", err)
	}

	if result.Status != 0 {
		return "", fmt.Errorf("Hubtel error %d: %s", result.Status, result.Message)
	}

	return result.Data.MessageID, nil
}

// ── ARKESEL API ───────────────────────────────────────────────────────────────

type arkeselSendRequest struct {
	Action  string   `json:"action"`
	APICode string   `json:"api_code"`
	To      []string `json:"to"`
	From    string   `json:"from"`
	SMS     string   `json:"sms"`
}

type arkeselSendResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    []struct {
		Recipient string `json:"recipient"`
		ID        string `json:"id"`
	} `json:"data"`
}

func (s *SMSService) sendViaArkesel(ctx context.Context, phone, message string) (string, error) {
	if s.arkeselAPIKey == "" {
		return "", fmt.Errorf("Arkesel credentials not configured")
	}

	payload := arkeselSendRequest{
		Action:  "send-sms",
		APICode: s.arkeselAPIKey,
		To:      []string{phone},
		From:    s.arkeselSenderID,
		SMS:     message,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://sms.arkesel.com/api/v2/sms/send", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("api-key", s.arkeselAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Arkesel request failed: %w", err)
	}
	defer resp.Body.Close()

	var result arkeselSendResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse Arkesel response: %w", err)
	}

	if strings.ToUpper(result.Status) != "SUCCESS" {
		return "", fmt.Errorf("Arkesel error: %s", result.Message)
	}

	if len(result.Data) > 0 {
		return result.Data[0].ID, nil
	}
	return "arkesel-sent", nil
}

// ── HELPERS ───────────────────────────────────────────────────────────────────

func (s *SMSService) updateSMSStatus(
	ctx context.Context,
	smsID uuid.UUID,
	status string,
	providerMsgID string,
	sendErr error,
) {
	if smsID == uuid.Nil {
		return
	}
	if sendErr != nil {
		s.db.Exec(ctx, `
			UPDATE sms_notifications SET
				status = 'FAILED', failed_at = NOW(),
				failure_reason = $1, updated_at = NOW()
			WHERE id = $2
		`, sendErr.Error(), smsID)
	} else {
		s.db.Exec(ctx, `
			UPDATE sms_notifications SET
				status = $1::sms_status, sent_at = NOW(),
				provider_message_id = $2, updated_at = NOW()
			WHERE id = $3
		`, status, providerMsgID, smsID)
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// normalisePhone converts Ghana phone numbers to +233 format
func normalisePhone(phone string) string {
	phone = regexp.MustCompile(`[^\d+]`).ReplaceAllString(phone, "")
	if strings.HasPrefix(phone, "0") && len(phone) == 10 {
		return "+233" + phone[1:]
	}
	if strings.HasPrefix(phone, "233") && len(phone) == 12 {
		return "+" + phone
	}
	return phone
}
