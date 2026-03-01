package gra

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	"go.uber.org/zap"
)

// GRABridgeService handles all communication with the GRA VSDC API v8.1
//
// Production flow:
//  1. Audit event is confirmed (field officer submits evidence)
//  2. SignInvoice() is called → GRA VSDC returns SDC-ID + QR code
//  3. QR code is stored in audit_events.gra_qr_code_url
//  4. Audit event is locked (is_locked = true) — immutable from this point
//
// Sandbox mode (GRA_SANDBOX_MODE=true):
//   - Returns realistic mock responses with locally-generated QR codes
//   - Used until GRA Letter of Intent is approved and live credentials issued
type GRABridgeService struct {
	client      *http.Client
	baseURL     string
	apiKey      string
	businessTIN string
	sandboxMode bool
	logger      *zap.Logger
}

// InvoiceRequest is the payload sent to GRA VSDC API
type InvoiceRequest struct {
	BusinessTIN   string        `json:"businessTin"`
	InvoiceNumber string        `json:"invoiceNumber"`
	InvoiceDate   string        `json:"invoiceDate"`
	CustomerTIN   string        `json:"customerTin,omitempty"`
	CustomerName  string        `json:"customerName"`
	LineItems     []InvoiceItem `json:"lineItems"`
	TotalAmount   float64       `json:"totalAmount"`
	VATAmount     float64       `json:"vatAmount"`
	Currency      string        `json:"currency"`
}

// InvoiceItem represents a line item in the GRA invoice
type InvoiceItem struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unitPrice"`
	TotalPrice  float64 `json:"totalPrice"`
	VATRate     float64 `json:"vatRate"`
	VATAmount   float64 `json:"vatAmount"`
}

// InvoiceResponse is the response from GRA VSDC API
type InvoiceResponse struct {
	Success        bool   `json:"success"`
	SDCID          string `json:"sdcId"`
	QRCodeURL      string `json:"qrCodeUrl"`
	QRCodeString   string `json:"qrCodeString"`
	QRCodeBase64   string `json:"qrCodeBase64,omitempty"` // PNG image as base64
	InvoiceNumber  string `json:"invoiceNumber"`
	SignedAt       string `json:"signedAt"`
	ErrorCode      string `json:"errorCode,omitempty"`
	ErrorMessage   string `json:"errorMessage,omitempty"`
}

// AuditInvoiceRequest is the GN-WAAS internal request to sign an audit event
type AuditInvoiceRequest struct {
	AuditEventID    string  `json:"audit_event_id"`
	AuditReference  string  `json:"audit_reference"`
	AccountNumber   string  `json:"account_number"`
	CustomerName    string  `json:"customer_name"`
	CustomerTIN     string  `json:"customer_tin,omitempty"`
	BillingPeriod   string  `json:"billing_period"`
	ConsumptionM3   float64 `json:"consumption_m3"`
	GWLBilledGHS    float64 `json:"gwl_billed_ghs"`
	ShadowBillGHS   float64 `json:"shadow_bill_ghs"`
	VariancePct     float64 `json:"variance_pct"`
	RecoveryAmtGHS  float64 `json:"recovery_amount_ghs"`
	VATAmountGHS    float64 `json:"vat_amount_ghs"`
	TotalInvoiceGHS float64 `json:"total_invoice_ghs"`
}

func NewGRABridgeService(
	baseURL, apiKey, businessTIN string,
	sandboxMode bool,
	logger *zap.Logger,
) *GRABridgeService {
	return &GRABridgeService{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:     baseURL,
		apiKey:      apiKey,
		businessTIN: businessTIN,
		sandboxMode: sandboxMode,
		logger:      logger,
	}
}

// SignAuditInvoice is the primary entry point for GN-WAAS audit compliance.
// It converts an audit event into a GRA VSDC invoice, signs it, and returns
// the QR code that locks the audit record.
func (g *GRABridgeService) SignAuditInvoice(
	ctx context.Context,
	audit *AuditInvoiceRequest,
) (*InvoiceResponse, error) {
	req := &InvoiceRequest{
		BusinessTIN:   g.businessTIN,
		InvoiceNumber: fmt.Sprintf("GNWAAS-%s", audit.AuditReference),
		InvoiceDate:   time.Now().Format("2006-01-02"),
		CustomerTIN:   audit.CustomerTIN,
		CustomerName:  audit.CustomerName,
		Currency:      "GHS",
		TotalAmount:   audit.TotalInvoiceGHS,
		VATAmount:     audit.VATAmountGHS,
		LineItems: []InvoiceItem{
			{
				Description: fmt.Sprintf("Water Audit Recovery — %s — Period: %s",
					audit.AccountNumber, audit.BillingPeriod),
				Quantity:   audit.ConsumptionM3,
				UnitPrice:  audit.RecoveryAmtGHS / audit.ConsumptionM3,
				TotalPrice: audit.RecoveryAmtGHS,
				VATRate:    20.0,
				VATAmount:  audit.VATAmountGHS,
			},
		},
	}

	return g.SignInvoice(ctx, req)
}

// SignInvoice submits an invoice to GRA VSDC for QR code signing.
// Implements exponential backoff retry (3 attempts) for transient failures.
func (g *GRABridgeService) SignInvoice(
	ctx context.Context,
	req *InvoiceRequest,
) (*InvoiceResponse, error) {
	if g.sandboxMode {
		resp := g.mockSignInvoice(req)
		// Generate a real QR code image even in sandbox mode
		qrBase64, err := generateQRCodeBase64(resp.QRCodeString)
		if err == nil {
			resp.QRCodeBase64 = qrBase64
		}
		return resp, nil
	}

	return g.signWithRetry(ctx, req, 3)
}

// signWithRetry implements exponential backoff for GRA API calls
func (g *GRABridgeService) signWithRetry(
	ctx context.Context,
	req *InvoiceRequest,
	maxAttempts int,
) (*InvoiceResponse, error) {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := g.liveSignInvoice(ctx, req)
		if err == nil {
			// Generate QR code image from the QR string
			if resp.QRCodeString != "" {
				qrBase64, qrErr := generateQRCodeBase64(resp.QRCodeString)
				if qrErr == nil {
					resp.QRCodeBase64 = qrBase64
				}
			}
			return resp, nil
		}

		lastErr = err
		if attempt < maxAttempts {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			g.logger.Warn("GRA API call failed, retrying",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
				zap.Error(err),
			)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
	}

	return nil, fmt.Errorf("GRA API failed after %d attempts: %w", maxAttempts, lastErr)
}

func (g *GRABridgeService) liveSignInvoice(
	ctx context.Context,
	req *InvoiceRequest,
) (*InvoiceResponse, error) {
	req.BusinessTIN = g.businessTIN
	req.Currency = "GHS"

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal invoice request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/invoices/sign", g.baseURL),
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.apiKey))
	httpReq.Header.Set("X-Business-TIN", g.businessTIN)
	httpReq.Header.Set("X-Client-ID", "GNWAAS-v1.0")

	g.logger.Info("Submitting invoice to GRA VSDC",
		zap.String("invoice_number", req.InvoiceNumber),
		zap.Float64("total_amount", req.TotalAmount),
		zap.String("url", fmt.Sprintf("%s/invoices/sign", g.baseURL)),
	)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("GRA API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read GRA response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("GRA API returned status %d: %s", resp.StatusCode, string(body))
	}

	var invoiceResp InvoiceResponse
	if err := json.Unmarshal(body, &invoiceResp); err != nil {
		return nil, fmt.Errorf("failed to parse GRA response: %w", err)
	}

	if !invoiceResp.Success {
		return nil, fmt.Errorf("GRA rejected invoice [%s]: %s",
			invoiceResp.ErrorCode, invoiceResp.ErrorMessage)
	}

	g.logger.Info("GRA invoice signed successfully",
		zap.String("invoice_number", req.InvoiceNumber),
		zap.String("sdc_id", invoiceResp.SDCID),
		zap.String("signed_at", invoiceResp.SignedAt),
	)

	return &invoiceResp, nil
}

// mockSignInvoice returns a realistic mock response for sandbox/development.
// Generates a real QR code image using the go-qrcode library.
func (g *GRABridgeService) mockSignInvoice(req *InvoiceRequest) *InvoiceResponse {
	sdcID := fmt.Sprintf("VSDC-SANDBOX-%s-%d", req.InvoiceNumber, time.Now().Unix())
	qrString := fmt.Sprintf("GRA|SANDBOX|%s|%.2f|GHS|%s|%s",
		req.InvoiceNumber,
		req.TotalAmount,
		time.Now().Format("20060102"),
		sdcID,
	)

	g.logger.Info("GRA SANDBOX MODE: Returning mock invoice response",
		zap.String("invoice_number", req.InvoiceNumber),
		zap.Float64("total_amount", req.TotalAmount),
		zap.String("sdc_id", sdcID),
	)

	return &InvoiceResponse{
		Success:       true,
		SDCID:         sdcID,
		QRCodeURL:     fmt.Sprintf("https://vsdc.gra.gov.gh/verify/%s", sdcID),
		QRCodeString:  qrString,
		InvoiceNumber: req.InvoiceNumber,
		SignedAt:      time.Now().UTC().Format(time.RFC3339),
	}
}

// VerifyQRCode verifies a GRA QR code / SDC-ID is valid
func (g *GRABridgeService) VerifyQRCode(ctx context.Context, sdcID string) (bool, string, error) {
	if g.sandboxMode {
		return true, "SANDBOX_VERIFIED", nil
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/invoices/verify/%s", g.baseURL, sdcID),
		nil,
	)
	if err != nil {
		return false, "", fmt.Errorf("failed to create verify request: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.apiKey))

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return false, "", fmt.Errorf("GRA verify request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		var result struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		json.Unmarshal(body, &result)
		return true, result.Status, nil
	}

	return false, "", fmt.Errorf("GRA verify returned status %d", resp.StatusCode)
}

// generateQRCodeBase64 generates a QR code PNG image and returns it as base64.
// The QR code encodes the GRA verification string for offline scanning.
func generateQRCodeBase64(content string) (string, error) {
	png, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil {
		return "", fmt.Errorf("QR code generation failed: %w", err)
	}
	return base64.StdEncoding.EncodeToString(png), nil
}

// GenerateQRCodePNG generates a QR code PNG image as bytes
func GenerateQRCodePNG(content string, size int) ([]byte, error) {
	return qrcode.Encode(content, qrcode.Medium, size)
}
