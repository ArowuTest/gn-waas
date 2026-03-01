// Package messaging provides NATS-based async messaging for GN-WAAS services.
//
// Architecture:
//   - Services publish events to NATS subjects
//   - Consumers subscribe and process events asynchronously
//   - This decouples services and prevents cascading failures
//
// Subject naming convention: gnwaas.<service>.<entity>.<event>
//
// Key subjects:
//   gnwaas.sentinel.anomaly.created     → Sentinel → API Gateway → Field Job dispatch
//   gnwaas.audit.event.completed        → API Gateway → GRA Bridge → sign invoice
//   gnwaas.cdc.sync.completed           → CDC Ingestor → Sentinel → trigger scan
//   gnwaas.gra.invoice.signed           → GRA Bridge → API Gateway → lock audit
//   gnwaas.ocr.reading.processed        → OCR Service → Sentinel → update meter reading
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// Subjects defines all NATS subjects used in GN-WAAS
const (
	// Sentinel publishes when a new anomaly flag is created
	SubjectAnomalyCreated = "gnwaas.sentinel.anomaly.created"

	// API Gateway publishes when an audit event is completed by a field officer
	SubjectAuditCompleted = "gnwaas.audit.event.completed"

	// CDC Ingestor publishes after each successful sync
	SubjectCDCSyncCompleted = "gnwaas.cdc.sync.completed"

	// GRA Bridge publishes when an invoice is signed (audit lock)
	SubjectInvoiceSigned = "gnwaas.gra.invoice.signed"

	// OCR Service publishes when a meter reading is processed
	SubjectOCRReadingProcessed = "gnwaas.ocr.reading.processed"

	// Sentinel publishes when a district scan completes
	SubjectSentinelScanCompleted = "gnwaas.sentinel.scan.completed"

	// Water balance computed
	SubjectWaterBalanceComputed = "gnwaas.sentinel.water_balance.computed"
)

// AnomalyCreatedEvent is published by Sentinel when a new anomaly is flagged
type AnomalyCreatedEvent struct {
	AnomalyFlagID  string    `json:"anomaly_flag_id"`
	DistrictID     string    `json:"district_id"`
	AccountID      string    `json:"account_id,omitempty"`
	AnomalyType    string    `json:"anomaly_type"`
	AlertLevel     string    `json:"alert_level"`
	EstimatedLoss  float64   `json:"estimated_loss_ghs"`
	CreatedAt      time.Time `json:"created_at"`
}

// AuditCompletedEvent is published when a field officer completes an audit
type AuditCompletedEvent struct {
	AuditEventID   string    `json:"audit_event_id"`
	AuditReference string    `json:"audit_reference"`
	AccountID      string    `json:"account_id"`
	OfficerID      string    `json:"officer_id"`
	TotalAmountGHS float64   `json:"total_amount_ghs"`
	VATAmountGHS   float64   `json:"vat_amount_ghs"`
	CompletedAt    time.Time `json:"completed_at"`
}

// CDCSyncCompletedEvent is published by CDC Ingestor after a sync
type CDCSyncCompletedEvent struct {
	SyncType        string    `json:"sync_type"` // accounts|billing|meters
	RecordsSynced   int       `json:"records_synced"`
	DistrictsAffected []string `json:"districts_affected"`
	CompletedAt     time.Time `json:"completed_at"`
}

// InvoiceSignedEvent is published by GRA Bridge when an invoice is signed
type InvoiceSignedEvent struct {
	AuditEventID  string    `json:"audit_event_id"`
	SDCID         string    `json:"sdc_id"`
	QRCodeURL     string    `json:"qr_code_url"`
	QRCodeBase64  string    `json:"qr_code_base64"`
	SignedAt      time.Time `json:"signed_at"`
}

// OCRReadingProcessedEvent is published by OCR Service
type OCRReadingProcessedEvent struct {
	AuditEventID  string    `json:"audit_event_id"`
	AccountID     string    `json:"account_id"`
	ReadingM3     float64   `json:"reading_m3"`
	Confidence    float64   `json:"confidence"`
	Status        string    `json:"status"` // SUCCESS|FAILED|MANUAL
	ProcessedAt   time.Time `json:"processed_at"`
}

// WaterBalanceComputedEvent is published when IWA water balance is computed
type WaterBalanceComputedEvent struct {
	DistrictID  string    `json:"district_id"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	NRWPercent  float64   `json:"nrw_percent"`
	IWAGrade    string    `json:"iwa_grade"`
	ILI         float64   `json:"ili"`
	ComputedAt  time.Time `json:"computed_at"`
}

// Client wraps the NATS connection with publish/subscribe helpers
type Client struct {
	nc     *nats.Conn
	logger *zap.Logger
}

// NewClient creates a new NATS client with reconnect logic
func NewClient(natsURL string, logger *zap.Logger) (*Client, error) {
	nc, err := nats.Connect(
		natsURL,
		nats.Name("gnwaas-service"),
		nats.MaxReconnects(10),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				logger.Warn("NATS disconnected", zap.Error(err))
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected", zap.String("url", nc.ConnectedUrl()))
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			logger.Error("NATS error", zap.String("subject", sub.Subject), zap.Error(err))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("NATS connect failed: %w", err)
	}

	logger.Info("NATS connected", zap.String("url", nc.ConnectedUrl()))
	return &Client{nc: nc, logger: logger}, nil
}

// NewClientOrNil creates a NATS client but returns nil (not an error) if NATS is unavailable.
// This allows services to run without NATS in development/testing.
func NewClientOrNil(natsURL string, logger *zap.Logger) *Client {
	if natsURL == "" {
		logger.Info("NATS_URL not set — async messaging disabled")
		return nil
	}
	client, err := NewClient(natsURL, logger)
	if err != nil {
		logger.Warn("NATS unavailable — running in sync-only mode", zap.Error(err))
		return nil
	}
	return client
}

// Publish publishes an event to a NATS subject
func (c *Client) Publish(subject string, event interface{}) error {
	if c == nil {
		return nil // NATS disabled — silently skip
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	if err := c.nc.Publish(subject, data); err != nil {
		return fmt.Errorf("NATS publish to %s failed: %w", subject, err)
	}
	c.logger.Debug("Published NATS event", zap.String("subject", subject))
	return nil
}

// Subscribe subscribes to a NATS subject with a typed handler
func (c *Client) Subscribe(subject string, handler func(data []byte) error) (*nats.Subscription, error) {
	if c == nil {
		return nil, nil // NATS disabled
	}
	sub, err := c.nc.Subscribe(subject, func(msg *nats.Msg) {
		if err := handler(msg.Data); err != nil {
			c.logger.Error("NATS message handler failed",
				zap.String("subject", subject),
				zap.Error(err),
			)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("NATS subscribe to %s failed: %w", subject, err)
	}
	c.logger.Info("Subscribed to NATS subject", zap.String("subject", subject))
	return sub, nil
}

// QueueSubscribe subscribes with a queue group (load-balanced across instances)
func (c *Client) QueueSubscribe(subject, queue string, handler func(data []byte) error) (*nats.Subscription, error) {
	if c == nil {
		return nil, nil
	}
	sub, err := c.nc.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
		if err := handler(msg.Data); err != nil {
			c.logger.Error("NATS queue handler failed",
				zap.String("subject", subject),
				zap.String("queue", queue),
				zap.Error(err),
			)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("NATS queue subscribe failed: %w", subject, err)
	}
	return sub, nil
}

// PublishAnomalyCreated is a typed helper for publishing anomaly events
func (c *Client) PublishAnomalyCreated(ctx context.Context, event *AnomalyCreatedEvent) error {
	return c.Publish(SubjectAnomalyCreated, event)
}

// PublishAuditCompleted is a typed helper for publishing audit completion events
func (c *Client) PublishAuditCompleted(ctx context.Context, event *AuditCompletedEvent) error {
	return c.Publish(SubjectAuditCompleted, event)
}

// PublishCDCSyncCompleted is a typed helper for CDC sync events
func (c *Client) PublishCDCSyncCompleted(ctx context.Context, event *CDCSyncCompletedEvent) error {
	return c.Publish(SubjectCDCSyncCompleted, event)
}

// PublishInvoiceSigned is a typed helper for GRA invoice signed events
func (c *Client) PublishInvoiceSigned(ctx context.Context, event *InvoiceSignedEvent) error {
	return c.Publish(SubjectInvoiceSigned, event)
}

// PublishOCRReadingProcessed is a typed helper for OCR events
func (c *Client) PublishOCRReadingProcessed(ctx context.Context, event *OCRReadingProcessedEvent) error {
	return c.Publish(SubjectOCRReadingProcessed, event)
}

// PublishWaterBalanceComputed is a typed helper for water balance events
func (c *Client) PublishWaterBalanceComputed(ctx context.Context, event *WaterBalanceComputedEvent) error {
	return c.Publish(SubjectWaterBalanceComputed, event)
}

// Close closes the NATS connection
func (c *Client) Close() {
	if c != nil && c.nc != nil {
		c.nc.Drain()
		c.nc.Close()
	}
}

// IsConnected returns true if the NATS connection is active
func (c *Client) IsConnected() bool {
	return c != nil && c.nc != nil && c.nc.IsConnected()
}
