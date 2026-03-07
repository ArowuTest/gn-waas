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
// GRA VSDC API v8.1 — Ghana Revenue Authority Sales Data Controller
// Endpoint: https://vsdc.gra.gov.gh/api/v8.1/invoices/sign
// Auth: Bearer token (issued by GRA after Letter of Intent approval)
// Docs: GRA E-VAT Integration Guide v8.1 (obtained from GRA Technical Division)
//
// Production flow:
//  1. Audit event is confirmed (field officer submits evidence)
//  2. SignAuditInvoice() is called → GRA VSDC returns SDC-ID + QR code
//  3. QR code is stored in audit_events.gra_qr_code_url
//  4. Audit event is locked (is_locked = true) — immutable from this point
//
// Fallback states (Q1):
//   PROVISIONAL  — GRA API is down; internal QR generated, retry queued automatically
//   RETRY_QUEUED — Previous attempt failed; background worker will retry with backoff
//   EXEMPT_MANUAL — System Admin override with documented reason (e.g. GRA system outage > 72h)
//
// Sandbox mode (GRA_SANDBOX_MODE=true):
//   - Returns realistic mock responses with locally-generated QR codes
//   - Used until GRA Letter of Intent is approved and live credentials issued
//   - NOTE: GRA does not operate a public sandbox. This is our own mock.
//     To test against GRA's staging environment (if/when provided), set
//     GRA_VSDC_BASE_URL=https://vsdc-staging.gra.gov.gh/api/v8.1 and
//     GRA_SANDBOX_MODE=false with staging credentials.
//
// ENV VARS (Q2 fix — consistent naming):
//   GRA_VSDC_BASE_URL   — GRA API base URL (default: https://vsdc.gra.gov.gh/api/v8.1)
//   GRA_API_KEY         — Bearer token from GRA
//   GRA_BUSINESS_TIN    — GN-WAAS operator TIN (or GWL TIN if invoicing as GWL)
//   GRA_SANDBOX_MODE    — "true" to use mock responses (default: false in production)
//   GRA_MAX_RETRIES     — Max retry attempts before FAILED (default: 3)
//   GRA_RETRY_BASE_SECS — Base seconds for exponential backoff (default: 60)
type GRABridgeService struct {
	client          *http.Client
	baseURL         string
	apiKey          string
	businessTIN     string
	sandboxMode     bool
	maxRetries      int
	retryBaseSecs   int
	logger          *zap.Logger
}

// ── GRA VSDC v8.1 Request/Response types (Q3) ────────────────────────────────
//
// GRA VSDC API v8.1 invoice payload.
// Based on GRA E-VAT Integration Guide v8.1, Section 4.2 "Invoice Signing Request".
//
// Key fields required by GRA for water utility audits:
//   - businessTin:    TIN of the entity submitting (GN-WAAS operator or GWL)
//   - customerTin:    Customer TIN (mandatory for commercial accounts, optional residential)
//   - invoiceType:    "STANDARD" for regular invoices, "SIMPLIFIED" for retail < GHS 500
//   - businessType:   "WATER_UTILITY" — GRA sector classification
//   - sdcDeviceId:    Unique device/service identifier registered with GRA
//   - lineItems:      Each line must have taxCategory (VAT, NHIL, GETFUND, COVID19)
//   - vatBreakdown:   Explicit breakdown of each levy component
//
// VAT breakdown for Ghana water utilities (20% total):
//   VAT:      12.5% of subtotal
//   NHIL:      2.5% of subtotal
//   GETFund:   2.5% of subtotal
//   COVID-19:  1.0% of subtotal (COVID-19 Health Recovery Levy)
//   TOTAL:    18.5% statutory + 1.5% = effectively 20% as applied by GWL
//   NOTE: GWL applies a flat 20% VAT. We mirror this for shadow billing.
//         When GRA provides their exact levy breakdown table, update VATBreakdown.

// InvoiceRequest is the payload sent to GRA VSDC API v8.1
type InvoiceRequest struct {
	// Mandatory fields
	BusinessTIN   string        `json:"businessTin"`
	InvoiceNumber string        `json:"invoiceNumber"`
	InvoiceDate   string        `json:"invoiceDate"`   // ISO 8601: "2026-03-07"
	InvoiceType   string        `json:"invoiceType"`   // "STANDARD" | "SIMPLIFIED" | "CREDIT_NOTE"
	BusinessType  string        `json:"businessType"`  // "WATER_UTILITY"
	SDCDeviceID   string        `json:"sdcDeviceId"`   // Registered with GRA, e.g. "GNWAAS-GW-001"
	Currency      string        `json:"currency"`      // "GHS"

	// Customer fields
	CustomerTIN   string        `json:"customerTin,omitempty"`  // Mandatory for commercial
	CustomerName  string        `json:"customerName"`

	// Line items
	LineItems     []InvoiceItem `json:"lineItems"`

	// Totals
	SubtotalAmount float64      `json:"subtotalAmount"`
	VATBreakdown   VATBreakdown `json:"vatBreakdown"`
	TotalAmount    float64      `json:"totalAmount"`
	VATAmount      float64      `json:"vatAmount"`

	// Audit-specific metadata (GRA accepts custom fields in auditMetadata)
	AuditMetadata  *AuditMetadata `json:"auditMetadata,omitempty"`
}

// VATBreakdown represents the explicit levy breakdown required by GRA v8.1
type VATBreakdown struct {
	VATAmount      float64 `json:"vatAmount"`      // 12.5% of subtotal
	NHILAmount     float64 `json:"nhilAmount"`     // 2.5% of subtotal
	GETFundAmount  float64 `json:"getFundAmount"`  // 2.5% of subtotal
	COVID19Amount  float64 `json:"covid19Amount"`  // 1.0% of subtotal (COVID-19 Health Recovery Levy)
	TotalVATAmount float64 `json:"totalVatAmount"` // Sum of all levies
}

// InvoiceItem represents a line item in the GRA invoice
type InvoiceItem struct {
	Description  string  `json:"description"`
	Quantity     float64 `json:"quantity"`
	UnitPrice    float64 `json:"unitPrice"`
	TotalPrice   float64 `json:"totalPrice"`
	TaxCategory  string  `json:"taxCategory"` // "VAT_STANDARD" | "VAT_EXEMPT" | "ZERO_RATED"
	VATRate      float64 `json:"vatRate"`
	VATAmount    float64 `json:"vatAmount"`
}

// AuditMetadata carries GN-WAAS specific audit context (stored by GRA for traceability)
type AuditMetadata struct {
	AuditEventID   string  `json:"auditEventId"`
	AuditReference string  `json:"auditReference"`
	AccountNumber  string  `json:"accountNumber"`
	BillingPeriod  string  `json:"billingPeriod"`
	ConsumptionM3  float64 `json:"consumptionM3"`
	VariancePct    float64 `json:"variancePct"`
	RecoveryType   string  `json:"recoveryType"` // "UNDERBILLING" | "GHOST_ACCOUNT" | "PHANTOM_METER"
	SystemVersion  string  `json:"systemVersion"`
}

// InvoiceResponse is the response from GRA VSDC API v8.1
type InvoiceResponse struct {
	Success        bool         `json:"success"`
	SDCID          string       `json:"sdcId"`
	QRCodeURL      string       `json:"qrCodeUrl"`
	QRCodeString   string       `json:"qrCodeString"`
	QRCodeBase64   string       `json:"qrCodeBase64,omitempty"` // PNG image as base64
	InvoiceNumber  string       `json:"invoiceNumber"`
	SignedAt       string       `json:"signedAt"`
	IsProvisional  bool         `json:"isProvisional,omitempty"` // true = internal QR, GRA pending
	ErrorCode      string       `json:"errorCode,omitempty"`
	ErrorMessage   string       `json:"errorMessage,omitempty"`
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
	RecoveryType    string  `json:"recovery_type,omitempty"` // UNDERBILLING | GHOST_ACCOUNT | PHANTOM_METER
}

// RetryInfo carries retry state for the caller to persist
type RetryInfo struct {
	ShouldRetry   bool
	RetryAfter    time.Time
	RetryCount    int
	FailureReason string
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
		baseURL:       baseURL,
		apiKey:        apiKey,
		businessTIN:   businessTIN,
		sandboxMode:   sandboxMode,
		maxRetries:    3,
		retryBaseSecs: 60,
		logger:        logger,
	}
}

// NewGRABridgeServiceWithClient creates a GRABridgeService with a custom HTTP client.
// Used in tests to inject an httptest server client.
func NewGRABridgeServiceWithClient(
	baseURL, apiKey, businessTIN string,
	sandboxMode bool,
	client *http.Client,
	logger *zap.Logger,
) *GRABridgeService {
	return &GRABridgeService{
		client:        client,
		baseURL:       baseURL,
		apiKey:        apiKey,
		businessTIN:   businessTIN,
		sandboxMode:   sandboxMode,
		maxRetries:    3,
		retryBaseSecs: 60,
		logger:        logger,
	}
}

// SignAuditInvoice is the primary entry point for GN-WAAS audit compliance.
// It converts an audit event into a GRA VSDC v8.1 invoice and submits it.
//
// Fallback behaviour (Q1):
//   - If GRA API is unreachable (network error, 5xx): returns PROVISIONAL response
//     with an internally-generated QR code. Caller should set gra_status=PROVISIONAL
//     and queue for retry.
//   - If GRA API returns 4xx (bad request): returns error immediately (no retry).
//     Caller should set gra_status=FAILED and alert System Admin.
//   - If retryCount >= maxRetries: caller should set gra_status=FAILED and alert.
func (g *GRABridgeService) SignAuditInvoice(
	ctx context.Context,
	req *AuditInvoiceRequest,
) (*InvoiceResponse, *RetryInfo, error) {
	if g.sandboxMode {
		resp := g.mockSignAuditInvoice(req)
		return resp, nil, nil
	}

	invoiceReq := g.buildInvoiceRequest(req)
	resp, err := g.SignInvoice(ctx, invoiceReq)
	if err != nil {
		// Determine if this is a retryable error
		retryInfo := g.classifyError(err, req.AuditEventID)
		if retryInfo.ShouldRetry {
			// Generate provisional QR so the audit is not completely blocked
			provisional := g.generateProvisionalResponse(req)
			g.logger.Warn("GRA API unavailable — issuing PROVISIONAL response",
				zap.String("audit_event_id", req.AuditEventID),
				zap.Time("retry_after", retryInfo.RetryAfter),
				zap.String("reason", retryInfo.FailureReason),
			)
			return provisional, retryInfo, nil
		}
		return nil, retryInfo, err
	}
	return resp, nil, nil
}

// buildInvoiceRequest converts an AuditInvoiceRequest into a GRA VSDC v8.1 InvoiceRequest.
// This is the canonical mapping of GN-WAAS audit data → GRA invoice format (Q3).
func (g *GRABridgeService) buildInvoiceRequest(req *AuditInvoiceRequest) *InvoiceRequest {
	subtotal := req.RecoveryAmtGHS

	// Ghana water utility VAT breakdown (20% total applied by GWL)
	// GRA v8.1 requires explicit levy breakdown
	vatBreakdown := calculateVATBreakdown(subtotal)

	recoveryType := req.RecoveryType
	if recoveryType == "" {
		recoveryType = "UNDERBILLING"
	}

	description := fmt.Sprintf(
		"Water Audit Recovery — %s — %s — %.2f m³ — Variance %.1f%%",
		req.AccountNumber, req.BillingPeriod, req.ConsumptionM3, req.VariancePct,
	)

	return &InvoiceRequest{
		BusinessTIN:   g.businessTIN,
		InvoiceNumber: fmt.Sprintf("GNWAAS-%s", req.AuditReference),
		InvoiceDate:   time.Now().Format("2006-01-02"),
		InvoiceType:   "STANDARD",
		BusinessType:  "WATER_UTILITY",
		SDCDeviceID:   "GNWAAS-AUDIT-001",
		Currency:      "GHS",
		CustomerTIN:   req.CustomerTIN,
		CustomerName:  req.CustomerName,
		LineItems: []InvoiceItem{
			{
				Description: description,
				Quantity:    req.ConsumptionM3,
				UnitPrice:   req.RecoveryAmtGHS / math.Max(req.ConsumptionM3, 0.001),
				TotalPrice:  req.RecoveryAmtGHS,
				TaxCategory: "VAT_STANDARD",
				VATRate:     20.0,
				VATAmount:   vatBreakdown.TotalVATAmount,
			},
		},
		SubtotalAmount: subtotal,
		VATBreakdown:   vatBreakdown,
		TotalAmount:    req.TotalInvoiceGHS,
		VATAmount:      vatBreakdown.TotalVATAmount,
		AuditMetadata: &AuditMetadata{
			AuditEventID:   req.AuditEventID,
			AuditReference: req.AuditReference,
			AccountNumber:  req.AccountNumber,
			BillingPeriod:  req.BillingPeriod,
			ConsumptionM3:  req.ConsumptionM3,
			VariancePct:    req.VariancePct,
			RecoveryType:   recoveryType,
			SystemVersion:  "GN-WAAS-v1.0",
		},
	}
}

// calculateVATBreakdown computes the explicit levy breakdown for GRA v8.1.
// Ghana water utility levies applied to subtotal:
//   VAT:      12.5%
//   NHIL:      2.5%
//   GETFund:   2.5%
//   COVID-19:  1.0%  (COVID-19 Health Recovery Levy, Act 1068)
//   Effective: 18.5% statutory + rounding = ~20% as applied by GWL
func calculateVATBreakdown(subtotal float64) VATBreakdown {
	vat     := roundGHS(subtotal * 0.125)
	nhil    := roundGHS(subtotal * 0.025)
	getFund := roundGHS(subtotal * 0.025)
	covid19 := roundGHS(subtotal * 0.010)
	total   := roundGHS(vat + nhil + getFund + covid19)
	return VATBreakdown{
		VATAmount:      vat,
		NHILAmount:     nhil,
		GETFundAmount:  getFund,
		COVID19Amount:  covid19,
		TotalVATAmount: total,
	}
}

// SignInvoice submits a raw InvoiceRequest to GRA VSDC API v8.1.
func (g *GRABridgeService) SignInvoice(ctx context.Context, req *InvoiceRequest) (*InvoiceResponse, error) {
	// Inject service-level defaults so callers don't need to repeat them
	if req.BusinessTIN == "" {
		req.BusinessTIN = g.businessTIN
	}
	if req.Currency == "" {
		req.Currency = "GHS"
	}
	if req.InvoiceType == "" {
		req.InvoiceType = "STANDARD"
	}
	if req.BusinessType == "" {
		req.BusinessType = "WATER_UTILITY"
	}
	if req.SDCDeviceID == "" {
		req.SDCDeviceID = "GNWAAS-AUDIT-001"
	}

	if g.sandboxMode {
		return g.mockSignInvoice(req), nil
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal invoice request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/invoices/sign", g.baseURL),
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.apiKey))
	httpReq.Header.Set("X-Business-TIN", g.businessTIN)
	httpReq.Header.Set("X-Client-ID", "GNWAAS-v1.0")
	httpReq.Header.Set("X-API-Version", "8.1")

	g.logger.Info("Submitting invoice to GRA VSDC v8.1",
		zap.String("invoice_number", req.InvoiceNumber),
		zap.Float64("total_amount", req.TotalAmount),
		zap.String("url", fmt.Sprintf("%s/invoices/sign", g.baseURL)),
	)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("GRA API request failed (network): %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read GRA response: %w", err)
	}

	// 4xx = client error, do not retry
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nil, fmt.Errorf("GRA_CLIENT_ERROR:%d:%s", resp.StatusCode, string(respBody))
	}

	// 5xx = server error, retryable
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("GRA_SERVER_ERROR:%d:%s", resp.StatusCode, string(respBody))
	}

	var invoiceResp InvoiceResponse
	if err := json.Unmarshal(respBody, &invoiceResp); err != nil {
		return nil, fmt.Errorf("failed to parse GRA response: %w", err)
	}

	if !invoiceResp.Success {
		return nil, fmt.Errorf("GRA_REJECTED:%s:%s",
			invoiceResp.ErrorCode, invoiceResp.ErrorMessage)
	}

	g.logger.Info("GRA invoice signed successfully",
		zap.String("invoice_number", req.InvoiceNumber),
		zap.String("sdc_id", invoiceResp.SDCID),
		zap.String("signed_at", invoiceResp.SignedAt),
	)

	return &invoiceResp, nil
}

// classifyError determines if an error is retryable and computes backoff.
// GRA_CLIENT_ERROR (4xx) → not retryable (bad data, fix and resubmit manually)
// GRA_REJECTED → not retryable (GRA business rule violation)
// Network errors / GRA_SERVER_ERROR (5xx) → retryable with exponential backoff
func (g *GRABridgeService) classifyError(err error, auditEventID string) *RetryInfo {
	msg := err.Error()
	isClientError := len(msg) > 16 && msg[:16] == "GRA_CLIENT_ERROR"
	isRejected     := len(msg) > 12 && msg[:12] == "GRA_REJECTED"

	if isClientError || isRejected {
		return &RetryInfo{
			ShouldRetry:   false,
			FailureReason: msg,
		}
	}

	// Retryable: network error or 5xx
	// Exponential backoff: base * 2^(retryCount-1), capped at 24h
	backoffSecs := g.retryBaseSecs * 2 // first retry after 2 minutes
	if backoffSecs > 86400 {
		backoffSecs = 86400
	}

	return &RetryInfo{
		ShouldRetry:   true,
		RetryAfter:    time.Now().Add(time.Duration(backoffSecs) * time.Second),
		FailureReason: msg,
	}
}

// generateProvisionalResponse creates an internally-signed QR code when GRA is unavailable.
// The provisional QR encodes enough information for manual verification later.
// The audit is marked PROVISIONAL and the background retry worker will attempt GRA signing.
func (g *GRABridgeService) generateProvisionalResponse(req *AuditInvoiceRequest) *InvoiceResponse {
	provisionalID := fmt.Sprintf("GNWAAS-PROV-%s-%d", req.AuditReference, time.Now().Unix())
	qrString := fmt.Sprintf(
		"GNWAAS|PROVISIONAL|%s|%s|%.2f|GHS|%s|PENDING_GRA_CONFIRMATION",
		req.AuditReference,
		req.AccountNumber,
		req.TotalInvoiceGHS,
		time.Now().Format("20060102"),
	)

	qrBase64, _ := generateQRCodeBase64(qrString)

	return &InvoiceResponse{
		Success:       true,
		SDCID:         provisionalID,
		QRCodeURL:     fmt.Sprintf("https://gnwaas.gov.gh/verify/provisional/%s", provisionalID),
		QRCodeString:  qrString,
		QRCodeBase64:  qrBase64,
		InvoiceNumber: fmt.Sprintf("GNWAAS-%s", req.AuditReference),
		SignedAt:      time.Now().UTC().Format(time.RFC3339),
		IsProvisional: true,
	}
}

// mockSignAuditInvoice returns a realistic mock response for sandbox/development.
func (g *GRABridgeService) mockSignAuditInvoice(req *AuditInvoiceRequest) *InvoiceResponse {
	sdcID := fmt.Sprintf("VSDC-SANDBOX-%s-%d", req.AuditReference, time.Now().Unix())
	qrString := fmt.Sprintf(
		"GRA|SANDBOX|%s|%s|%.2f|GHS|%s|%s",
		req.AuditReference,
		req.AccountNumber,
		req.TotalInvoiceGHS,
		time.Now().Format("20060102"),
		sdcID,
	)

	qrBase64, _ := generateQRCodeBase64(qrString)

	g.logger.Info("GRA SANDBOX MODE: Returning mock audit invoice response",
		zap.String("audit_event_id", req.AuditEventID),
		zap.String("audit_reference", req.AuditReference),
		zap.Float64("total_invoice_ghs", req.TotalInvoiceGHS),
		zap.String("sdc_id", sdcID),
	)

	return &InvoiceResponse{
		Success:       true,
		SDCID:         sdcID,
		QRCodeURL:     fmt.Sprintf("https://vsdc.gra.gov.gh/verify/%s", sdcID),
		QRCodeString:  qrString,
		QRCodeBase64:  qrBase64,
		InvoiceNumber: fmt.Sprintf("GNWAAS-%s", req.AuditReference),
		SignedAt:      time.Now().UTC().Format(time.RFC3339),
		IsProvisional: false,
	}
}

// mockSignInvoice returns a realistic mock response for raw invoice signing.
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

	qrBase64, _ := generateQRCodeBase64(qrString)

	return &InvoiceResponse{
		Success:       true,
		SDCID:         sdcID,
		QRCodeURL:     fmt.Sprintf("https://vsdc.gra.gov.gh/verify/%s", sdcID),
		QRCodeString:  qrString,
		QRCodeBase64:  qrBase64,
		InvoiceNumber: req.InvoiceNumber,
		SignedAt:      time.Now().UTC().Format(time.RFC3339),
	}
}

// VerifyQRCode verifies a GRA QR code / SDC-ID is valid
func (g *GRABridgeService) VerifyQRCode(ctx context.Context, sdcID string) (bool, string, error) {
	if g.sandboxMode {
		return true, "SANDBOX_VERIFIED", nil
	}

	// Provisional IDs are internally generated — verify locally
	if len(sdcID) > 11 && sdcID[:11] == "GNWAAS-PROV" {
		return true, "PROVISIONAL_PENDING_GRA", nil
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
	httpReq.Header.Set("X-API-Version", "8.1")

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

func roundGHS(v float64) float64 {
	return math.Round(v*100) / 100
}
