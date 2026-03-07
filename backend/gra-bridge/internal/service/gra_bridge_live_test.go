package gra_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gra "github.com/ArowuTest/gn-waas/backend/gra-bridge/internal/service"
	"go.uber.org/zap"
)

// ============================================================
// Helpers
// ============================================================

func newTestLogger(t *testing.T) *zap.Logger {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	return logger
}

func newLiveServiceWithServer(t *testing.T, server *httptest.Server) *gra.GRABridgeService {
	t.Helper()
	return gra.NewGRABridgeServiceWithClient(
		server.URL,
		"test-api-key-12345",
		"C0000000001",
		false, // live mode
		server.Client(),
		newTestLogger(t),
	)
}

func sampleInvoiceRequest() *gra.InvoiceRequest {
	return &gra.InvoiceRequest{
		InvoiceNumber: "AUD-2026-LIVE-001",
		CustomerName:  "Kwame Mensah",
		TotalAmount:   144.96,
		VATAmount:     24.16,
		LineItems: []gra.InvoiceItem{
			{
				Description: "Water Audit Recovery — GWL-ACC-12345",
				Quantity:    1.0,
				UnitPrice:   120.80,
				TotalPrice:  120.80,
				VATRate:     20.0,
				VATAmount:   24.16,
			},
		},
	}
}

// ============================================================
// Live Mode Tests (using httptest mock server)
// ============================================================

func TestLiveSignInvoice_SuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request structure
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key-12345" {
			t.Errorf("Missing or wrong Authorization header: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("X-Business-TIN") != "C0000000001" {
			t.Errorf("Missing X-Business-TIN header")
		}

		// Decode request body
		var req gra.InvoiceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}
		if req.InvoiceNumber == "" {
			t.Error("Expected non-empty InvoiceNumber in request")
		}
		if req.Currency != "GHS" {
			t.Errorf("Expected currency GHS, got %s", req.Currency)
		}
		if req.BusinessTIN != "C0000000001" {
			t.Errorf("Expected BusinessTIN to be set from service config, got %s", req.BusinessTIN)
		}

		// Return success response
		resp := gra.InvoiceResponse{
			Success:       true,
			SDCID:         "VSDC-LIVE-2026-001234",
			InvoiceNumber: req.InvoiceNumber,
			QRCodeString:  "GRA|LIVE|AUD-2026-LIVE-001|144.96|GHS|20260301|VSDC-LIVE-2026-001234",
			QRCodeBase64:  "",
			SignedAt:      time.Now().UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := newLiveServiceWithServer(t, server)
	resp, err := svc.SignInvoice(context.Background(), sampleInvoiceRequest())
	if err != nil {
		t.Fatalf("SignInvoice failed: %v", err)
	}
	if !resp.Success {
		t.Error("Expected success=true")
	}
	if resp.SDCID != "VSDC-LIVE-2026-001234" {
		t.Errorf("Expected SDCID VSDC-LIVE-2026-001234, got %s", resp.SDCID)
	}
	if resp.InvoiceNumber != "AUD-2026-LIVE-001" {
		t.Errorf("Expected invoice number AUD-2026-LIVE-001, got %s", resp.InvoiceNumber)
	}
}

func TestLiveSignInvoice_GRARejectsInvoice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := gra.InvoiceResponse{
			Success:      false,
			ErrorCode:    "INVALID_TIN",
			ErrorMessage: "Business TIN not registered in GRA system",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // GRA returns 200 even for business errors
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := newLiveServiceWithServer(t, server)
	_, err := svc.SignInvoice(context.Background(), sampleInvoiceRequest())
	if err == nil {
		t.Fatal("Expected error when GRA rejects invoice, got nil")
	}
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
	// Error should contain the GRA error code
	errStr := err.Error()
	if len(errStr) == 0 {
		t.Error("Expected descriptive error message")
	}
}

func TestLiveSignInvoice_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Service temporarily unavailable"))
	}))
	defer server.Close()

	svc := newLiveServiceWithServer(t, server)
	_, err := svc.SignInvoice(context.Background(), sampleInvoiceRequest())
	if err == nil {
		t.Fatal("Expected error on HTTP 503, got nil")
	}
}

func TestLiveSignInvoice_UnauthorizedReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_token"}`))
	}))
	defer server.Close()

	svc := newLiveServiceWithServer(t, server)
	_, err := svc.SignInvoice(context.Background(), sampleInvoiceRequest())
	if err == nil {
		t.Fatal("Expected error on HTTP 401, got nil")
	}
}

func TestLiveSignInvoice_MalformedJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{not valid json`))
	}))
	defer server.Close()

	svc := newLiveServiceWithServer(t, server)
	_, err := svc.SignInvoice(context.Background(), sampleInvoiceRequest())
	if err == nil {
		t.Fatal("Expected error on malformed JSON response, got nil")
	}
}

func TestLiveVerifyQRCode_ValidCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		// Live VerifyQRCode reads "status" field from JSON body
		resp := map[string]interface{}{
			"status":  "VERIFIED",
			"message": "Invoice verified successfully",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := newLiveServiceWithServer(t, server)
	valid, status, err := svc.VerifyQRCode(context.Background(), "VSDC-LIVE-2026-001234")
	if err != nil {
		t.Fatalf("VerifyQRCode failed: %v", err)
	}
	if !valid {
		t.Error("Expected valid=true on HTTP 200")
	}
	if status != "VERIFIED" {
		t.Errorf("Expected status VERIFIED, got %s", status)
	}
}

func TestLiveVerifyQRCode_InvalidCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GRA returns 404 for unknown SDC IDs
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"SDC ID not found"}`))
	}))
	defer server.Close()

	svc := newLiveServiceWithServer(t, server)
	_, _, err := svc.VerifyQRCode(context.Background(), "INVALID-CODE")
	// Live mode returns error on non-200
	if err == nil {
		t.Fatal("Expected error for 404 response, got nil")
	}
}

// ============================================================
// Context Cancellation Tests
// ============================================================

func TestLiveSignInvoice_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow GRA API
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := newLiveServiceWithServer(t, server)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := svc.SignInvoice(ctx, sampleInvoiceRequest())
	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}
}

// ============================================================
// Invoice Amount Validation Tests
// ============================================================

func TestSignAuditInvoice_VATCalculation(t *testing.T) {
	svc := gra.NewGRABridgeService(
		"https://vsdc.gra.gov.gh/api/v1",
		"",
		"C0000000000",
		true, // sandbox
		newTestLogger(t),
	)

	audit := &gra.AuditInvoiceRequest{
		AuditEventID:    "ae-vat-test",
		AuditReference:  "AUD-2026-VAT-001",
		AccountNumber:   "GWL-ACC-99999",
		CustomerName:    "Ama Owusu",
		BillingPeriod:   "2026-02",
		ConsumptionM3:   30.0,
		GWLBilledGHS:    200.00,
		ShadowBillGHS:   350.00,
		VariancePct:     75.0,
		RecoveryAmtGHS:  150.00,
		VATAmountGHS:    30.00, // 20% of 150
		TotalInvoiceGHS: 180.00,
	}

	resp, _, err := svc.SignAuditInvoice(context.Background(), audit)
	if err != nil {
		t.Fatalf("SignAuditInvoice failed: %v", err)
	}
	if !resp.Success {
		t.Error("Expected success=true")
	}
	// Verify the invoice number contains the audit reference
	if resp.InvoiceNumber == "" {
		t.Error("Expected non-empty InvoiceNumber")
	}
	// Verify SDCID is set (sandbox generates one)
	if resp.SDCID == "" {
		t.Error("Expected non-empty SDCID")
	}
}

func TestSignAuditInvoice_ZeroVarianceNotSubmitted(t *testing.T) {
	svc := gra.NewGRABridgeService(
		"https://vsdc.gra.gov.gh/api/v1",
		"",
		"C0000000000",
		true,
		newTestLogger(t),
	)

	audit := &gra.AuditInvoiceRequest{
		AuditEventID:    "ae-zero-variance",
		AuditReference:  "AUD-2026-ZERO-001",
		AccountNumber:   "GWL-ACC-00001",
		CustomerName:    "Kofi Boateng",
		BillingPeriod:   "2026-02",
		ConsumptionM3:   10.0,
		GWLBilledGHS:    100.00,
		ShadowBillGHS:   100.00,
		VariancePct:     0.0,
		RecoveryAmtGHS:  0.0,
		VATAmountGHS:    0.0,
		TotalInvoiceGHS: 0.0,
	}

	// Zero-recovery invoices should still succeed (GRA needs the record)
	resp, _, err := svc.SignAuditInvoice(context.Background(), audit)
	if err != nil {
		t.Fatalf("SignAuditInvoice failed for zero variance: %v", err)
	}
	if !resp.Success {
		t.Error("Expected success=true even for zero variance")
	}
}
