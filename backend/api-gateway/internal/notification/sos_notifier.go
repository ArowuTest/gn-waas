package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

// SOSNotifier sends SOS alerts to supervisors via multiple channels.
// Channels are configured via environment variables:
//   - NOTIFY_WEBHOOK_URL: Slack/Teams/custom webhook (primary)
//   - NOTIFY_SMS_URL: SMS gateway URL (secondary)
//   - NOTIFY_SMS_API_KEY: SMS gateway API key
//
// If no channels are configured, the SOS is logged only (development mode).
type SOSNotifier struct {
	webhookURL string
	smsURL     string
	smsAPIKey  string
	httpClient *http.Client
	logger     *zap.Logger
}

// SOSAlert is the payload sent to notification channels
type SOSAlert struct {
	JobID       string    `json:"job_id"`
	OfficerID   string    `json:"officer_id"`
	OfficerName string    `json:"officer_name"`
	OfficerLat  float64   `json:"officer_lat"`
	OfficerLng  float64   `json:"officer_lng"`
	TriggeredAt time.Time `json:"triggered_at"`
	MapsURL     string    `json:"maps_url"`
}

func NewSOSNotifier(logger *zap.Logger) *SOSNotifier {
	return &SOSNotifier{
		webhookURL: os.Getenv("NOTIFY_WEBHOOK_URL"),
		smsURL:     os.Getenv("NOTIFY_SMS_URL"),
		smsAPIKey:  os.Getenv("NOTIFY_SMS_API_KEY"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

// SendSOSAlert dispatches the SOS alert to all configured channels.
// It is non-blocking — errors are logged but do not fail the HTTP response.
func (n *SOSNotifier) SendSOSAlert(ctx context.Context, alert SOSAlert) {
	alert.TriggeredAt = time.Now().UTC()
	alert.MapsURL = fmt.Sprintf(
		"https://maps.google.com/?q=%.6f,%.6f",
		alert.OfficerLat, alert.OfficerLng,
	)

	n.logger.Warn("🚨 SOS ALERT DISPATCHING",
		zap.String("job_id", alert.JobID),
		zap.String("officer_id", alert.OfficerID),
		zap.String("officer_name", alert.OfficerName),
		zap.Float64("lat", alert.OfficerLat),
		zap.Float64("lng", alert.OfficerLng),
		zap.String("maps_url", alert.MapsURL),
	)

	// Send to webhook (Slack/Teams/custom)
	if n.webhookURL != "" {
		go n.sendWebhook(alert)
	} else {
		n.logger.Warn("NOTIFY_WEBHOOK_URL not configured — SOS logged only (set in production)")
	}

	// Send SMS via gateway
	if n.smsURL != "" && n.smsAPIKey != "" {
		go n.sendSMS(alert)
	} else {
		n.logger.Warn("NOTIFY_SMS_URL/NOTIFY_SMS_API_KEY not configured — SMS disabled")
	}
}

// sendWebhook posts a Slack-compatible webhook message
func (n *SOSNotifier) sendWebhook(alert SOSAlert) {
	payload := map[string]interface{}{
		"text": fmt.Sprintf("🚨 *GN-WAAS SOS ALERT*"),
		"attachments": []map[string]interface{}{
			{
				"color": "#FF0000",
				"fields": []map[string]string{
					{"title": "Officer", "value": alert.OfficerName, "short": "true"},
					{"title": "Job ID", "value": alert.JobID, "short": "true"},
					{"title": "GPS Location", "value": alert.MapsURL, "short": "false"},
					{"title": "Time", "value": alert.TriggeredAt.Format("02 Jan 2006 15:04:05 UTC"), "short": "true"},
				},
				"footer": "GN-WAAS Field Officer Safety System",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		n.webhookURL,
		bytes.NewBuffer(body),
	)
	if err != nil {
		n.logger.Error("Failed to create webhook request", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		n.logger.Error("Webhook SOS notification failed", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	n.logger.Info("SOS webhook notification sent",
		zap.String("job_id", alert.JobID),
		zap.Int("status", resp.StatusCode),
	)
}

// sendSMS sends an SMS via a configurable SMS gateway (e.g. Hubtel, Twilio)
func (n *SOSNotifier) sendSMS(alert SOSAlert) {
	message := fmt.Sprintf(
		"GN-WAAS SOS ALERT: Officer %s needs assistance. Job: %s. Location: %s. Time: %s",
		alert.OfficerName,
		alert.JobID,
		alert.MapsURL,
		alert.TriggeredAt.Format("15:04 UTC"),
	)

	payload := map[string]string{
		"message":   message,
		"recipient": "SUPERVISOR", // Resolved by SMS gateway from officer's district
		"sender":    "GNWAAS",
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		n.smsURL,
		bytes.NewBuffer(body),
	)
	if err != nil {
		n.logger.Error("Failed to create SMS request", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", n.smsAPIKey))

	resp, err := n.httpClient.Do(req)
	if err != nil {
		n.logger.Error("SMS SOS notification failed", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	n.logger.Info("SOS SMS notification sent",
		zap.String("job_id", alert.JobID),
		zap.Int("status", resp.StatusCode),
	)
}
