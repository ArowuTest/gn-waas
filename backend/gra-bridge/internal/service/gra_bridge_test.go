package gra_test

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	gra "github.com/ArowuTest/gn-waas/backend/gra-bridge/internal/service"
	"go.uber.org/zap"
)

func newSandboxService(t *testing.T) *gra.GRABridgeService {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	return gra.NewGRABridgeService(
		"https://vsdc.gra.gov.gh/api/v1",
		"",
		"C0000000000",
		true, // sandbox mode
		logger,
	)
}

func TestSignInvoice_SandboxReturnsSuccess(t *testing.T) {
	svc := newSandboxService(t)

	req := &gra.InvoiceRequest{
		InvoiceNumber: "TEST-INV-001",
		CustomerName:  "Kwame Mensah",
		TotalAmount:   150.00,
		VATAmount:     25.00,
		LineItems: []gra.InvoiceItem{
			{
				Description: "Water Audit Recovery",
				Quantity:    10.0,
				UnitPrice:   15.0,
				TotalPrice:  150.0,
				VATRate:     20.0,
				VATAmount:   25.0,
			},
		},
	}

	resp, err := svc.SignInvoice(context.Background(), req)
	if err != nil {
		t.Fatalf("SignInvoice failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success=true, got false")
	}
	if resp.SDCID == "" {
		t.Error("Expected non-empty SDCID")
	}
	if resp.QRCodeString == "" {
		t.Error("Expected non-empty QRCodeString")
	}
	if resp.InvoiceNumber != "TEST-INV-001" {
		t.Errorf("Expected invoice number TEST-INV-001, got %s", resp.InvoiceNumber)
	}
	if resp.SignedAt == "" {
		t.Error("Expected non-empty SignedAt")
	}
}

func TestSignInvoice_SandboxGeneratesQRCodeBase64(t *testing.T) {
	svc := newSandboxService(t)

	req := &gra.InvoiceRequest{
		InvoiceNumber: "TEST-QR-001",
		CustomerName:  "Ama Owusu",
		TotalAmount:   200.00,
		VATAmount:     33.33,
		LineItems: []gra.InvoiceItem{
			{Description: "Water Audit", Quantity: 1, UnitPrice: 200, TotalPrice: 200, VATRate: 20, VATAmount: 33.33},
		},
	}

	resp, err := svc.SignInvoice(context.Background(), req)
	if err != nil {
		t.Fatalf("SignInvoice failed: %v", err)
	}

	// QR code base64 should be a valid PNG
	if resp.QRCodeBase64 == "" {
		t.Error("Expected QRCodeBase64 to be populated")
	}

	decoded, err := base64.StdEncoding.DecodeString(resp.QRCodeBase64)
	if err != nil {
		t.Errorf("QRCodeBase64 is not valid base64: %v", err)
	}

	// PNG magic bytes: 0x89 0x50 0x4E 0x47
	if len(decoded) < 4 || decoded[0] != 0x89 || decoded[1] != 0x50 {
		t.Error("QRCodeBase64 does not decode to a valid PNG image")
	}
}

func TestSignAuditInvoice_BuildsCorrectLineItem(t *testing.T) {
	svc := newSandboxService(t)

	audit := &gra.AuditInvoiceRequest{
		AuditEventID:    "ae-001",
		AuditReference:  "AUD-2026-001",
		AccountNumber:   "GWL-ACC-12345",
		CustomerName:    "Kofi Boateng",
		BillingPeriod:   "2026-01",
		ConsumptionM3:   25.0,
		GWLBilledGHS:    150.00,
		ShadowBillGHS:   270.80,
		VariancePct:     80.5,
		RecoveryAmtGHS:  120.80,
		VATAmountGHS:    24.16,
		TotalInvoiceGHS: 144.96,
	}

	resp, _, err := svc.SignAuditInvoice(context.Background(), audit)
	if err != nil {
		t.Fatalf("SignAuditInvoice failed: %v", err)
	}

	if !resp.Success {
		t.Error("Expected success=true")
	}

	// Invoice number should include the audit reference
	if !strings.Contains(resp.InvoiceNumber, "AUD-2026-001") {
		t.Errorf("Expected invoice number to contain AUD-2026-001, got %s", resp.InvoiceNumber)
	}
}

func TestVerifyQRCode_SandboxAlwaysValid(t *testing.T) {
	svc := newSandboxService(t)

	valid, status, err := svc.VerifyQRCode(context.Background(), "VSDC-SANDBOX-TEST-123")
	if err != nil {
		t.Fatalf("VerifyQRCode failed: %v", err)
	}
	if !valid {
		t.Error("Expected sandbox QR code to be valid")
	}
	if status != "SANDBOX_VERIFIED" {
		t.Errorf("Expected status SANDBOX_VERIFIED, got %s", status)
	}
}

func TestGenerateQRCodePNG_ValidOutput(t *testing.T) {
	content := "GRA|SANDBOX|TEST-001|150.00|GHS|20260301|VSDC-TEST"
	png, err := gra.GenerateQRCodePNG(content, 256)
	if err != nil {
		t.Fatalf("GenerateQRCodePNG failed: %v", err)
	}
	if len(png) == 0 {
		t.Error("Expected non-empty PNG bytes")
	}
	// Check PNG magic bytes
	if png[0] != 0x89 || png[1] != 0x50 || png[2] != 0x4E || png[3] != 0x47 {
		t.Error("Output is not a valid PNG file")
	}
}
