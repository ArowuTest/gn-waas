package gra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// GRABridgeService handles all communication with the GRA VSDC API v8.1
// In sandbox mode (GRA_SANDBOX_MODE=true), all calls return mock responses
// Production credentials must be obtained via GRA Letter of Intent process
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
	Success       bool   `json:"success"`
	SDCID         string `json:"sdcId"`
	QRCodeURL     string `json:"qrCodeUrl"`
	QRCodeString  string `json:"qrCodeString"`
	InvoiceNumber string `json:"invoiceNumber"`
	SignedAt       string `json:"signedAt"`
	ErrorCode     string `json:"errorCode,omitempty"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
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

// SignInvoice submits an invoice to GRA VSDC for QR code signing
// This is the GRA compliance lock - audit is only complete after QR code receipt
func (g *GRABridgeService) SignInvoice(
	ctx context.Context,
	req *InvoiceRequest,
) (*InvoiceResponse, error) {

	if g.sandboxMode {
		return g.mockSignInvoice(req), nil
	}

	return g.liveSignInvoice(ctx, req)
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

	g.logger.Info("Submitting invoice to GRA VSDC",
		zap.String("invoice_number", req.InvoiceNumber),
		zap.Float64("total_amount", req.TotalAmount),
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

	g.logger.Info("GRA invoice signed successfully",
		zap.String("invoice_number", req.InvoiceNumber),
		zap.String("sdc_id", invoiceResp.SDCID),
	)

	return &invoiceResp, nil
}

// mockSignInvoice returns a realistic mock response for sandbox/development
func (g *GRABridgeService) mockSignInvoice(req *InvoiceRequest) *InvoiceResponse {
	g.logger.Info("GRA SANDBOX MODE: Returning mock invoice response",
		zap.String("invoice_number", req.InvoiceNumber),
		zap.Float64("total_amount", req.TotalAmount),
	)

	return &InvoiceResponse{
		Success:       true,
		SDCID:         fmt.Sprintf("VSDC-SANDBOX-%s-%d", req.InvoiceNumber, time.Now().Unix()),
		QRCodeURL:     fmt.Sprintf("https://vsdc.gra.gov.gh/verify/SANDBOX-%s", req.InvoiceNumber),
		QRCodeString:  fmt.Sprintf("GRA|SANDBOX|%s|%.2f|GHS|%s", req.InvoiceNumber, req.TotalAmount, time.Now().Format("20060102")),
		InvoiceNumber: req.InvoiceNumber,
		SignedAt:       time.Now().UTC().Format(time.RFC3339),
	}
}

// VerifyQRCode verifies a GRA QR code is valid
func (g *GRABridgeService) VerifyQRCode(ctx context.Context, sdcID string) (bool, error) {
	if g.sandboxMode {
		return true, nil
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/invoices/verify/%s", g.baseURL, sdcID),
		nil,
	)
	if err != nil {
		return false, fmt.Errorf("failed to create verify request: %w", err)
	}

	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.apiKey))

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("GRA verify request failed: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}
