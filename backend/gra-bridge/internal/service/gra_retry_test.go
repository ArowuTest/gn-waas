package gra_test

// Integration tests for GRA Bridge retry logic.
//
// Tests verify: error classification (retryable vs non-retryable),
// provisional response when GRA is unreachable, sandbox pass-through,
// and concurrent invocation safety. All sandbox tests never hit the real GRA API.

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	gra "github.com/ArowuTest/gn-waas/backend/gra-bridge/internal/service"
	"go.uber.org/zap"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func newSandboxGRA(t *testing.T) *gra.GRABridgeService {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	return gra.NewGRABridgeService(
		"https://vsdc.gra.gov.gh/api/v1",
		"",
		"C0000000000",
		true, // sandbox=true — never hits real GRA
		logger,
	)
}

func makeAuditReq(ref string, total float64) *gra.AuditInvoiceRequest {
	return &gra.AuditInvoiceRequest{
		AuditEventID:    fmt.Sprintf("evt-%s", ref),
		AuditReference:  ref,
		AccountNumber:   "GWL-ACC-TEST-001",
		CustomerName:    "Kwame Mensah",
		BillingPeriod:   "2026-01",
		ConsumptionM3:   15.0,
		GWLBilledGHS:    total * 0.9,
		ShadowBillGHS:   total,
		VariancePct:     10.0,
		RecoveryAmtGHS:  total * 0.1,
		VATAmountGHS:    total * 0.1667,
		TotalInvoiceGHS: total,
		RecoveryType:    "UNDERBILLING",
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 1: Successful first attempt (sandbox)
// ─────────────────────────────────────────────────────────────────────────────

func TestSignAuditInvoice_SandboxSuccess(t *testing.T) {
	t.Parallel()

	svc := newSandboxGRA(t)
	req := makeAuditReq("AUDIT-RETRY-001", 180.00)

	resp, _, err := svc.SignAuditInvoice(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error on sandbox sign: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got false")
	}
	if resp.SDCID == "" {
		t.Error("expected non-empty SDCID on success")
	}
	if resp.QRCodeString == "" {
		t.Error("expected non-empty QRCodeString on success")
	}
	if resp.SignedAt == "" {
		t.Error("expected non-empty SignedAt on success")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 2: Sandbox mode never hits real GRA even with broken URL
// ─────────────────────────────────────────────────────────────────────────────

func TestSignAuditInvoice_SandboxNeverCallsRealGRA(t *testing.T) {
	t.Parallel()

	logger, _ := zap.NewDevelopment()
	svc := gra.NewGRABridgeService(
		"https://this-url-does-not-exist.invalid/api",
		"bad-key",
		"C0000000000",
		true, // sandbox mode: real URL is irrelevant
		logger,
	)

	req := makeAuditReq("AUDIT-SANDBOX-002", 90.00)
	resp, _, err := svc.SignAuditInvoice(context.Background(), req)
	if err != nil {
		t.Fatalf("sandbox mode should not return error with broken URL: %v", err)
	}
	if !resp.Success {
		t.Error("sandbox mode should always return success regardless of URL")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 3: 4xx client error — NOT retryable
// ─────────────────────────────────────────────────────────────────────────────

func TestClassifyError_ClientError_NotRetryable(t *testing.T) {
	t.Parallel()

	svc := newSandboxGRA(t)
	clientErr := errors.New("GRA_CLIENT_ERROR: invalid tin number format")
	info := svc.ClassifyError(clientErr, "audit-001")

	if info.ShouldRetry {
		t.Errorf("GRA_CLIENT_ERROR (4xx) must NOT be retried — fix the request first")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 4: GRA_REJECTED — NOT retryable (business rule violation)
// ─────────────────────────────────────────────────────────────────────────────

func TestClassifyError_GRARejected_NotRetryable(t *testing.T) {
	t.Parallel()

	svc := newSandboxGRA(t)
	rejectedErr := errors.New("GRA_REJECTED: duplicate invoice number")
	info := svc.ClassifyError(rejectedErr, "audit-002")

	if info.ShouldRetry {
		t.Errorf("GRA_REJECTED must NOT be retried — manual intervention required")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 5: Network error — IS retryable
// ─────────────────────────────────────────────────────────────────────────────

func TestClassifyError_NetworkError_IsRetryable(t *testing.T) {
	t.Parallel()

	svc := newSandboxGRA(t)
	networkErr := errors.New("connection refused: dial tcp: connect")
	info := svc.ClassifyError(networkErr, "audit-003")

	if !info.ShouldRetry {
		t.Errorf("network error SHOULD be retried with backoff")
	}
	if info.RetryAfter.IsZero() {
		t.Error("RetryAfter must be set for retryable errors")
	}
	if !info.RetryAfter.After(time.Now()) {
		t.Error("RetryAfter must be in the future (backoff delay)")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 6: 5xx server error — IS retryable
// ─────────────────────────────────────────────────────────────────────────────

func TestClassifyError_ServerError_IsRetryable(t *testing.T) {
	t.Parallel()

	svc := newSandboxGRA(t)
	serverErr := errors.New("GRA_SERVER_ERROR: internal server error 503")
	info := svc.ClassifyError(serverErr, "audit-004")

	if !info.ShouldRetry {
		t.Errorf("GRA_SERVER_ERROR (5xx) SHOULD be retried")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 7: RetryAfter is always in the future (exponential backoff)
// ─────────────────────────────────────────────────────────────────────────────

func TestClassifyError_RetryAfterInFuture(t *testing.T) {
	t.Parallel()

	svc := newSandboxGRA(t)
	before := time.Now()

	info := svc.ClassifyError(errors.New("timeout: GRA_SERVER_ERROR"), "audit-005")
	if !info.ShouldRetry {
		t.Skip("error classified as non-retryable, skip backoff check")
	}

	if !info.RetryAfter.After(before) {
		t.Errorf("RetryAfter (%v) must be after call time (%v)", info.RetryAfter, before)
	}
	// Minimum 30s backoff — free-tier GRA takes time to recover
	minBackoff := before.Add(30 * time.Second)
	if info.RetryAfter.Before(minBackoff) {
		t.Errorf("backoff too short — expected ≥30s, got %v", info.RetryAfter.Sub(before))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 8: Provisional response when GRA is unreachable
// ─────────────────────────────────────────────────────────────────────────────

func TestSignAuditInvoice_ProvisionalWhenGRAUnreachable(t *testing.T) {
	t.Parallel()

	// Point at a dead server with non-sandbox mode
	logger, _ := zap.NewDevelopment()
	svc := gra.NewGRABridgeService(
		"https://this-host-does-not-exist-gnwaas.invalid/api",
		"no-key",
		"C0000000000",
		false, // live mode: will try to connect, fail, then go provisional
		logger,
	)

	req := makeAuditReq("AUDIT-PROV-001", 200.00)
	resp, _, err := svc.SignAuditInvoice(context.Background(), req)

	// Must NOT return an error — provisional response keeps audit flowing
	if err != nil {
		t.Fatalf("unreachable GRA should produce provisional response, not error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response for unreachable GRA")
	}
	if !resp.IsProvisional {
		t.Errorf("expected IsProvisional=true, got false — audit chain broken")
	}
	if resp.SDCID == "" {
		t.Error("provisional SDCID must not be empty")
	}
	if resp.QRCodeString == "" {
		t.Error("provisional response must include an internal QR code")
	}
	if !resp.Success {
		t.Error("provisional response must be success=true (audit continues offline)")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 9: Concurrent invoice signing — race-condition check
// ─────────────────────────────────────────────────────────────────────────────

func TestSignAuditInvoice_Concurrent(t *testing.T) {
	t.Parallel()

	svc := newSandboxGRA(t)
	const n = 10

	type result struct {
		resp *gra.InvoiceResponse
		err  error
	}
	results := make(chan result, n)

	for i := 0; i < n; i++ {
		i := i
		go func() {
			req := makeAuditReq(fmt.Sprintf("AUDIT-CONC-%03d", i), float64(100+i*10))
			resp, _, err := svc.SignAuditInvoice(context.Background(), req)
			results <- result{resp, err}
		}()
	}

	passed := 0
	for i := 0; i < n; i++ {
		r := <-results
		if r.err != nil {
			t.Errorf("concurrent invoice failed: %v", r.err)
			continue
		}
		if !r.resp.Success {
			t.Errorf("concurrent invoice returned success=false: %v", r.resp)
			continue
		}
		passed++
	}

	if passed < n {
		t.Errorf("expected %d/%d concurrent invoices to succeed, got %d", n, n, passed)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 10: Table-driven error classification
// ─────────────────────────────────────────────────────────────────────────────

func TestClassifyError_TableDriven(t *testing.T) {
	t.Parallel()

	svc := newSandboxGRA(t)

	cases := []struct {
		name          string
		errMsg        string
		wantRetryable bool
	}{
		{"4xx client error", "GRA_CLIENT_ERROR: bad tin", false},
		{"GRA rejection", "GRA_REJECTED: duplicate invoice", false},
		{"5xx server error", "GRA_SERVER_ERROR: internal error", true},
		{"network timeout", "context deadline exceeded", true},
		{"connection refused", "connect: connection refused", true},
		{"generic failure", "some unknown failure", true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			info := svc.ClassifyError(errors.New(tc.errMsg), "audit-table")
			if info.ShouldRetry != tc.wantRetryable {
				t.Errorf("%q: want ShouldRetry=%v, got %v",
					tc.errMsg, tc.wantRetryable, info.ShouldRetry)
			}
		})
	}
}
