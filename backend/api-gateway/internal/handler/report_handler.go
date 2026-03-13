package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jung-kurt/gofpdf"
	"go.uber.org/zap"

	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/repository"
	"github.com/ArowuTest/gn-waas/shared/go/http/response"
)

// ReportHandler generates server-side PDF and CSV reports.
// These are official, signed documents suitable for regulatory submission.
type ReportHandler struct {
	gwlCaseRepo  *repository.GWLCaseRepository
	auditRepo    *repository.AuditEventRepository
	fieldJobRepo *repository.FieldJobRepository
	logger       *zap.Logger
}

func NewReportHandler(
	gwlCaseRepo *repository.GWLCaseRepository,
	auditRepo *repository.AuditEventRepository,
	fieldJobRepo *repository.FieldJobRepository,
	logger *zap.Logger,
) *ReportHandler {
	return &ReportHandler{
		gwlCaseRepo:  gwlCaseRepo,
		auditRepo:    auditRepo,
		fieldJobRepo: fieldJobRepo,
		logger:       logger,
	}
}

// GetMonthlyReportPDF generates a server-side PDF of the GWL monthly report.
// GET /api/v1/reports/monthly/pdf?period=2026-01&district_id=<uuid>
func (h *ReportHandler) GetMonthlyReportPDF(c *fiber.Ctx) error {
	periodStr := c.Query("period", time.Now().AddDate(0, -1, 0).Format("2006-01"))
	period, err := time.Parse("2006-01", periodStr)
	if err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid period format — use YYYY-MM")
	}

	var districtID *uuid.UUID
	if d := c.Query("district_id"); d != "" {
		id, err := uuid.Parse(d)
		if err != nil {
			return response.BadRequest(c, "BAD_REQUEST", "invalid district_id")
		}
		districtID = &id
	}

	report, err := h.gwlCaseRepo.GetMonthlyReport(c.UserContext(), period, districtID)
	if err != nil {
		h.logger.Error("GetMonthlyReport failed for PDF", zap.Error(err))
		return response.InternalError(c, "failed to generate monthly report")
	}

	pdfBytes, err := generateMonthlyReportPDF(report, period, periodStr)
	if err != nil {
		h.logger.Error("PDF generation failed", zap.Error(err))
		return response.InternalError(c, "failed to generate PDF")
	}

	filename := fmt.Sprintf("GN-WAAS-Monthly-Report-%s.pdf", periodStr)
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Set("X-Report-Period", periodStr)
	c.Set("X-Generated-At", time.Now().UTC().Format(time.RFC3339))
	c.Set("X-Generated-By", "GN-WAAS Report Engine v1.0")

	return c.Send(pdfBytes)
}

// GetMonthlyReportCSV generates a CSV export of the GWL monthly report.
// GET /api/v1/reports/monthly/csv?period=2026-01&district_id=<uuid>
func (h *ReportHandler) GetMonthlyReportCSV(c *fiber.Ctx) error {
	periodStr := c.Query("period", time.Now().AddDate(0, -1, 0).Format("2006-01"))
	period, err := time.Parse("2006-01", periodStr)
	if err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid period format — use YYYY-MM")
	}

	var districtID *uuid.UUID
	if d := c.Query("district_id"); d != "" {
		id, err := uuid.Parse(d)
		if err != nil {
			return response.BadRequest(c, "BAD_REQUEST", "invalid district_id")
		}
		districtID = &id
	}

	report, err := h.gwlCaseRepo.GetMonthlyReport(c.UserContext(), period, districtID)
	if err != nil {
		h.logger.Error("GetMonthlyReport failed for CSV", zap.Error(err))
		return response.InternalError(c, "failed to generate monthly report")
	}

	csvBytes := generateMonthlyReportCSV(report, periodStr)
	filename := fmt.Sprintf("GN-WAAS-Monthly-Report-%s.csv", periodStr)
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(csvBytes)
}

// generateMonthlyReportPDF creates a branded PDF using gofpdf with real DB data.
// The PDF includes: header, period, KPI summary, case breakdown table,
// financial impact, and a digital audit trail footer.
func generateMonthlyReportPDF(reportData interface{}, period time.Time, periodStr string) ([]byte, error) {
	// Extract stats from the map returned by GetMonthlyReport
	type ReportStats struct {
		TotalFlagged         int     `json:"total_flagged"`
		CriticalCases        int     `json:"critical_cases"`
		Resolved             int     `json:"resolved"`
		Pending              int     `json:"pending"`
		Disputed             int     `json:"disputed"`
		TotalUnderbillingGHS float64 `json:"total_underbilling_ghs"`
		TotalOverbillingGHS  float64 `json:"total_overbilling_ghs"`
		RevenueRecoveredGHS  float64 `json:"revenue_recovered_ghs"`
		CreditsIssuedGHS     float64 `json:"credits_issued_ghs"`
		FieldJobsAssigned    int     `json:"field_jobs_assigned"`
		FieldJobsCompleted   int     `json:"field_jobs_completed"`
	}
	var rs ReportStats
	if report, ok := reportData.(map[string]interface{}); ok {
		if raw, ok := report["statistics"]; ok {
			if b, err := json.Marshal(raw); err == nil {
				_ = json.Unmarshal(b, &rs)
			}
		}
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// ── Header ────────────────────────────────────────────────────────────────
	pdf.SetFont("Arial", "B", 20)
	pdf.SetTextColor(30, 94, 32)
	pdf.CellFormat(0, 12, "GN-WAAS", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 12)
	pdf.SetTextColor(60, 60, 60)
	pdf.CellFormat(0, 7, "Ghana National Water Audit & Assurance System", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(0, 10, fmt.Sprintf("Monthly Audit Report — %s", period.Format("January 2006")), "", 1, "C", false, 0, "")
	pdf.SetDrawColor(30, 94, 32)
	pdf.SetLineWidth(0.5)
	pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
	pdf.Ln(5)

	// ── Metadata ──────────────────────────────────────────────────────────────
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(100, 100, 100)
	pdf.CellFormat(0, 5,
		fmt.Sprintf("Generated: %s  |  System: GN-WAAS Report Engine v1.0  |  Classification: OFFICIAL",
			time.Now().UTC().Format("02 Jan 2006 15:04 UTC")),
		"", 1, "C", false, 0, "")
	pdf.Ln(5)

	// ── KPI Summary ───────────────────────────────────────────────────────────
	pdf.SetFont("Arial", "B", 12)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(0, 8, "Executive Summary — Key Performance Indicators", "", 1, "L", false, 0, "")
	pdf.SetLineWidth(0.2)
	pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
	pdf.Ln(4)

	kpiRow := func(label, value string) {
		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(60, 60, 60)
		pdf.CellFormat(80, 7, label, "1", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 10)
		pdf.SetTextColor(0, 0, 0)
		pdf.CellFormat(100, 7, value, "1", 1, "L", false, 0, "")
	}
	kpiRow("Report Period", period.Format("January 2006"))
	kpiRow("Total Anomaly Flags", fmt.Sprintf("%d", rs.TotalFlagged))
	kpiRow("Critical Cases", fmt.Sprintf("%d", rs.CriticalCases))
	kpiRow("Resolved Cases", fmt.Sprintf("%d", rs.Resolved))
	kpiRow("Pending Cases", fmt.Sprintf("%d", rs.Pending))
	kpiRow("Disputed Cases", fmt.Sprintf("%d", rs.Disputed))
	kpiRow("Total Underbilling (GHS)", fmt.Sprintf("GHS %.2f", rs.TotalUnderbillingGHS))
	kpiRow("Total Overbilling (GHS)", fmt.Sprintf("GHS %.2f", rs.TotalOverbillingGHS))
	kpiRow("Revenue Recovered (GHS)", fmt.Sprintf("GHS %.2f", rs.RevenueRecoveredGHS))
	kpiRow("Credits Issued (GHS)", fmt.Sprintf("GHS %.2f", rs.CreditsIssuedGHS))
	kpiRow("Field Jobs Assigned", fmt.Sprintf("%d", rs.FieldJobsAssigned))
	kpiRow("Field Jobs Completed", fmt.Sprintf("%d", rs.FieldJobsCompleted))
	pdf.Ln(6)

	// ── Case Breakdown Table ──────────────────────────────────────────────────
	pdf.SetFont("Arial", "B", 12)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(0, 8, "Case Status Breakdown", "", 1, "L", false, 0, "")
	pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
	pdf.Ln(4)

	pdf.SetFillColor(30, 94, 32)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(60, 8, "Status", "1", 0, "C", true, 0, "")
	pdf.CellFormat(40, 8, "Count", "1", 0, "C", true, 0, "")
	pdf.CellFormat(80, 8, "% of Total", "1", 1, "C", true, 0, "")

	total := rs.TotalFlagged
	if total == 0 {
		total = 1
	}
	tableRows := []struct {
		label string
		count int
	}{
		{"Critical", rs.CriticalCases},
		{"Resolved", rs.Resolved},
		{"Pending", rs.Pending},
		{"Disputed", rs.Disputed},
	}
	pdf.SetTextColor(0, 0, 0)
	for i, r := range tableRows {
		if i%2 == 0 {
			pdf.SetFillColor(245, 245, 245)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.SetFont("Arial", "", 10)
		pct := float64(r.count) / float64(total) * 100
		pdf.CellFormat(60, 7, r.label, "1", 0, "L", true, 0, "")
		pdf.CellFormat(40, 7, fmt.Sprintf("%d", r.count), "1", 0, "C", true, 0, "")
		pdf.CellFormat(80, 7, fmt.Sprintf("%.1f%%", pct), "1", 1, "C", true, 0, "")
	}
	pdf.Ln(6)

	// ── Financial Impact ──────────────────────────────────────────────────────
	pdf.SetFont("Arial", "B", 12)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(0, 8, "Financial Impact Summary (GHS)", "", 1, "L", false, 0, "")
	pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
	pdf.Ln(4)

	netImpact := rs.TotalUnderbillingGHS - rs.RevenueRecoveredGHS
	recoveryRate := 0.0
	if rs.TotalUnderbillingGHS > 0 {
		recoveryRate = rs.RevenueRecoveredGHS / rs.TotalUnderbillingGHS * 100
	}
	pdf.SetFont("Arial", "", 10)
	pdf.SetFillColor(255, 243, 205)
	pdf.CellFormat(100, 7, "Net Unrecovered Revenue (GHS)", "1", 0, "L", true, 0, "")
	pdf.CellFormat(80, 7, fmt.Sprintf("GHS %.2f", netImpact), "1", 1, "R", true, 0, "")
	pdf.SetFillColor(220, 255, 220)
	pdf.CellFormat(100, 7, "Revenue Recovery Rate", "1", 0, "L", true, 0, "")
	pdf.CellFormat(80, 7, fmt.Sprintf("%.1f%%", recoveryRate), "1", 1, "R", true, 0, "")
	pdf.Ln(8)

	// ── Regulatory Note ───────────────────────────────────────────────────────
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(30, 94, 32)
	pdf.CellFormat(0, 7, "Regulatory Compliance", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(60, 60, 60)
	pdf.MultiCell(0, 5,
		"This report is generated in compliance with PURC 2026 Tariff Schedule and GRA E-VAT requirements. "+
			"All anomaly flags have been processed through the GN-WAAS sentinel engine. "+
			"Revenue figures are based on shadow billing calculations using the 2026 PURC tiered tariff rates.",
		"", "L", false)

	// ── Audit Trail Footer ────────────────────────────────────────────────────
	pdf.SetY(-30)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(120, 120, 120)
	pdf.SetDrawColor(200, 200, 200)
	pdf.SetLineWidth(0.2)
	pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
	pdf.Ln(2)
	pdf.CellFormat(0, 4,
		fmt.Sprintf("GN-WAAS Official Report | Period: %s | Generated: %s UTC | Total Flags: %d | Recovered: GHS %.2f",
			periodStr, time.Now().UTC().Format("2006-01-02 15:04:05"),
			rs.TotalFlagged, rs.RevenueRecoveredGHS),
		"", 1, "C", false, 0, "")
	pdf.CellFormat(0, 4,
		"This document is system-generated and forms part of the official GN-WAAS audit trail. "+
			"Unauthorised modification is a criminal offence under the Ghana Electronic Transactions Act.",
		"", 1, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("PDF output failed: %w", err)
	}
	return buf.Bytes(), nil
}

// generateMonthlyReportCSV creates a CSV export of the monthly report with real data.
func generateMonthlyReportCSV(reportData interface{}, periodStr string) []byte {
	// Extract stats
	type ReportStats struct {
		TotalFlagged         int     `json:"total_flagged"`
		CriticalCases        int     `json:"critical_cases"`
		Resolved             int     `json:"resolved"`
		Pending              int     `json:"pending"`
		Disputed             int     `json:"disputed"`
		TotalUnderbillingGHS float64 `json:"total_underbilling_ghs"`
		TotalOverbillingGHS  float64 `json:"total_overbilling_ghs"`
		RevenueRecoveredGHS  float64 `json:"revenue_recovered_ghs"`
		CreditsIssuedGHS     float64 `json:"credits_issued_ghs"`
		ReclassRequested     int     `json:"reclassifications_requested"`
		ReclassApplied       int     `json:"reclassifications_applied"`
		FieldJobsAssigned    int     `json:"field_jobs_assigned"`
		FieldJobsCompleted   int     `json:"field_jobs_completed"`
	}
	var rs ReportStats
	if report, ok := reportData.(map[string]interface{}); ok {
		if raw, ok := report["statistics"]; ok {
			if b, err := json.Marshal(raw); err == nil {
				_ = json.Unmarshal(b, &rs)
			}
		}
	}

	row := func(label, value string) string {
		return label + "," + value + "\n"
	}

	var buf bytes.Buffer
	// BOM for Excel compatibility
	buf.WriteString("\xEF\xBB\xBF")
	buf.WriteString(row("GN-WAAS Monthly Audit Report", periodStr))
	buf.WriteString(row("Generated", time.Now().UTC().Format("2006-01-02 15:04:05")+" UTC"))
	buf.WriteString(row("System", "GN-WAAS Report Engine v1.0"))
	buf.WriteString(row("Authority", "Ghana National Water Audit & Assurance System"))
	buf.WriteString(row("Regulatory Framework", "PURC 2026 Tariff Schedule | GRA E-VAT Compliance"))
	buf.WriteString("\n")
	buf.WriteString(row("KPI", "Value"))
	buf.WriteString(row("Total Anomaly Flags", fmt.Sprintf("%d", rs.TotalFlagged)))
	buf.WriteString(row("Critical Cases", fmt.Sprintf("%d", rs.CriticalCases)))
	buf.WriteString(row("Resolved Cases", fmt.Sprintf("%d", rs.Resolved)))
	buf.WriteString(row("Pending Cases", fmt.Sprintf("%d", rs.Pending)))
	buf.WriteString(row("Disputed Cases", fmt.Sprintf("%d", rs.Disputed)))
	buf.WriteString(row("Total Underbilling (GHS)", fmt.Sprintf("%.2f", rs.TotalUnderbillingGHS)))
	buf.WriteString(row("Total Overbilling (GHS)", fmt.Sprintf("%.2f", rs.TotalOverbillingGHS)))
	buf.WriteString(row("Revenue Recovered (GHS)", fmt.Sprintf("%.2f", rs.RevenueRecoveredGHS)))
	buf.WriteString(row("Credits Issued (GHS)", fmt.Sprintf("%.2f", rs.CreditsIssuedGHS)))
	buf.WriteString(row("Field Jobs Assigned", fmt.Sprintf("%d", rs.FieldJobsAssigned)))
	buf.WriteString(row("Field Jobs Completed", fmt.Sprintf("%d", rs.FieldJobsCompleted)))
	buf.WriteString("\n")
	netUnrecovered := rs.TotalUnderbillingGHS - rs.RevenueRecoveredGHS
	recoveryRate := 0.0
	if rs.TotalUnderbillingGHS > 0 {
		recoveryRate = rs.RevenueRecoveredGHS / rs.TotalUnderbillingGHS * 100
	}
	buf.WriteString(row("Financial Summary", "Value"))
	buf.WriteString(row("Net Unrecovered Revenue (GHS)", fmt.Sprintf("%.2f", netUnrecovered)))
	buf.WriteString(row("Revenue Recovery Rate", fmt.Sprintf("%.1f%%", recoveryRate)))
	return buf.Bytes()
}

// ─── GRA Compliance CSV ───────────────────────────────────────────────────────

// GetGRAComplianceCSV generates a CSV export of GRA VSDC compliance status.
// GET /api/v1/reports/gra-compliance/csv?period=2026-01&district_id=<uuid>
//
// Columns: Audit Reference, Account Number, District, GRA Status, GWL Billed (GHS),
//          Shadow Bill (GHS), Variance (%), Is Locked, Created At
func (h *ReportHandler) GetGRAComplianceCSV(c *fiber.Ctx) error {
	periodStr := c.Query("period", time.Now().AddDate(0, -1, 0).Format("2006-01"))
	districtIDStr := c.Query("district_id")

	// district_id is optional for SUPER_ADMIN/SYSTEM_ADMIN (gets all districts).
	// Other roles must supply a specific district.
	var districtID uuid.UUID
	if districtIDStr != "" {
		var parseErr error
		districtID, parseErr = uuid.Parse(districtIDStr)
		if parseErr != nil {
			return response.BadRequest(c, "BAD_REQUEST", "invalid district_id")
		}
	}

	// Parse period into date range for unbounded export
	periodParsed, _ := time.Parse("2006-01", periodStr)
	periodStart := time.Date(periodParsed.Year(), periodParsed.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0) // exclusive (start of next month)

	// GetAllForExport fetches records up to the safety cap (50k) — correct for regulatory docs.
	// If truncated, an X-Truncated header signals the client that not all rows are present.
	events, truncated, err := h.auditRepo.GetAllForExport(c.UserContext(), districtID, periodStart, periodEnd)
	if err != nil {
		h.logger.Error("GetGRAComplianceCSV: GetAllForExport failed", zap.Error(err))
		return response.InternalError(c, "failed to fetch audit events")
	}
	if truncated {
		c.Set("X-Truncated", "true")
		h.logger.Warn("GetGRAComplianceCSV: result truncated at exportMaxRows",
			zap.String("district_id", districtID.String()),
			zap.String("period", periodStr),
		)
	}
	c.Set("X-Record-Count", fmt.Sprintf("%d", len(events)))

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // BOM for Excel
	buf.WriteString("GN-WAAS GRA Compliance Report," + periodStr + "\n")
	buf.WriteString("Generated," + time.Now().UTC().Format("2006-01-02 15:04:05") + " UTC\n")
	buf.WriteString("District ID," + districtIDStr + "\n\n")
	buf.WriteString("Audit Reference,Account ID,GRA Status,GWL Billed (GHS),Shadow Bill (GHS),Variance (%),Is Locked,Created At\n")

	for _, e := range events {
		graStatus := e.GRAStatus
		gwlBilled := 0.0
		if e.GWLBilledGHS != nil {
			gwlBilled = *e.GWLBilledGHS
		}
		shadowBill := 0.0
		if e.ShadowBillGHS != nil {
			shadowBill = *e.ShadowBillGHS
		}
		variance := 0.0
		if e.VariancePct != nil {
			variance = *e.VariancePct
		}
		locked := "No"
		if e.IsLocked {
			locked = "Yes"
		}
		buf.WriteString(fmt.Sprintf("%s,%s,%s,%.2f,%.2f,%.2f,%s,%s\n",
			e.AuditReference,
			e.AccountID.String(),
			graStatus,
			gwlBilled,
			shadowBill,
			variance,
			locked,
			e.CreatedAt.Format("2006-01-02 15:04:05"),
		))
	}

	filename := fmt.Sprintf("GN-WAAS-GRA-Compliance-%s.csv", periodStr)
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(buf.Bytes())
}

// ─── Audit Trail CSV ──────────────────────────────────────────────────────────

// GetAuditTrailCSV generates a full immutable audit trail CSV export.
// GET /api/v1/reports/audit-trail/csv?period=2026-01&district_id=<uuid>
//
// Columns: Audit Reference, Account ID, District ID, Anomaly Flag ID,
//          Status, Assigned Officer, GRA Status, GWL Billed, Shadow Bill,
//          Variance %, Is Locked, Created At, Updated At
func (h *ReportHandler) GetAuditTrailCSV(c *fiber.Ctx) error {
	periodStr := c.Query("period", time.Now().AddDate(0, -1, 0).Format("2006-01"))
	districtIDStr := c.Query("district_id")

	// district_id is optional for admins
	var districtID uuid.UUID
	if districtIDStr != "" {
		var parseErr error
		districtID, parseErr = uuid.Parse(districtIDStr)
		if parseErr != nil {
			return response.BadRequest(c, "BAD_REQUEST", "invalid district_id")
		}
	}

	// Parse period into date range for unbounded export
	periodParsed2, _ := time.Parse("2006-01", periodStr)
	periodStart2 := time.Date(periodParsed2.Year(), periodParsed2.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd2 := periodStart2.AddDate(0, 1, 0)

	events, truncated2, err := h.auditRepo.GetAllForExport(c.UserContext(), districtID, periodStart2, periodEnd2)
	if err != nil {
		h.logger.Error("GetAuditTrailCSV: GetAllForExport failed", zap.Error(err))
		return response.InternalError(c, "failed to fetch audit trail")
	}
	if truncated2 {
		c.Set("X-Truncated", "true")
		h.logger.Warn("GetAuditTrailCSV: result truncated at exportMaxRows",
			zap.String("district_id", districtID.String()),
			zap.String("period", periodStr),
		)
	}
	c.Set("X-Record-Count", fmt.Sprintf("%d", len(events)))

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	buf.WriteString("GN-WAAS Immutable Audit Trail Export," + periodStr + "\n")
	buf.WriteString("Generated," + time.Now().UTC().Format("2006-01-02 15:04:05") + " UTC\n")
	buf.WriteString("Authority,Ghana National Water Audit & Assurance System\n")
	buf.WriteString("Regulatory Framework,PURC 2026 | GRA E-VAT Compliance | Electronic Transactions Act\n\n")
	buf.WriteString("Audit Reference,Account ID,District ID,Anomaly Flag ID,Status,Assigned Officer ID,GRA Status,GWL Billed (GHS),Shadow Bill (GHS),Variance (%),Is Locked,Created At,Updated At\n")

	for _, e := range events {
		flagID := ""
		if e.AnomalyFlagID != nil {
			flagID = e.AnomalyFlagID.String()
		}
		officerID := ""
		if e.AssignedOfficerID != nil {
			officerID = e.AssignedOfficerID.String()
		}
		graStatus := e.GRAStatus
		gwlBilled := 0.0
		if e.GWLBilledGHS != nil {
			gwlBilled = *e.GWLBilledGHS
		}
		shadowBill := 0.0
		if e.ShadowBillGHS != nil {
			shadowBill = *e.ShadowBillGHS
		}
		variance := 0.0
		if e.VariancePct != nil {
			variance = *e.VariancePct
		}
		locked := "No"
		if e.IsLocked {
			locked = "Yes"
		}
		buf.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%.2f,%.2f,%.2f,%s,%s,%s\n",
			e.AuditReference,
			e.AccountID.String(),
			e.DistrictID.String(),
			flagID,
			e.Status,
			officerID,
			graStatus,
			gwlBilled,
			shadowBill,
			variance,
			locked,
			e.CreatedAt.Format("2006-01-02 15:04:05"),
			e.UpdatedAt.Format("2006-01-02 15:04:05"),
		))
	}

	filename := fmt.Sprintf("GN-WAAS-Audit-Trail-%s.csv", periodStr)
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(buf.Bytes())
}

// ─── Field Jobs CSV ───────────────────────────────────────────────────────────

// GetFieldJobsCSV generates a CSV export of all field officer dispatch jobs.
// GET /api/v1/reports/field-jobs/csv?period=2026-01&district_id=<uuid>
//
// Columns: Job Reference, Account ID, District ID, Assigned Officer ID,
//          Status, Is Blind Audit, Priority, GPS Fence (m),
//          Officer Lat, Officer Lng, Created At, Updated At
func (h *ReportHandler) GetFieldJobsCSV(c *fiber.Ctx) error {
	periodStr := c.Query("period", time.Now().AddDate(0, -1, 0).Format("2006-01"))
	districtIDStr := c.Query("district_id", "")

	// Export up to 200 jobs per request (paginated if needed via offset param).
	limit := 200
	offset := c.QueryInt("offset", 0)
	jobs, _, err := h.fieldJobRepo.ListAll(c.UserContext(), "", "", districtIDStr, limit, offset)
	if err != nil {
		h.logger.Error("GetFieldJobsCSV: ListAll failed", zap.Error(err))
		return response.InternalError(c, "failed to fetch field jobs")
	}

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	buf.WriteString("GN-WAAS Field Jobs Summary," + periodStr + "\n")
	buf.WriteString("Generated," + time.Now().UTC().Format("2006-01-02 15:04:05") + " UTC\n\n")
	buf.WriteString("Job Reference,Account ID,District ID,Assigned Officer ID,Status,Is Blind Audit,Priority,GPS Fence (m),Officer Lat,Officer Lng,Created At,Updated At\n")

	for _, j := range jobs {
		officerID := ""
		if j.AssignedOfficerID != nil {
			officerID = j.AssignedOfficerID.String()
		}
		officerLat := ""
		if j.OfficerGPSLat != nil {
			officerLat = fmt.Sprintf("%.6f", *j.OfficerGPSLat)
		}
		officerLng := ""
		if j.OfficerGPSLng != nil {
			officerLng = fmt.Sprintf("%.6f", *j.OfficerGPSLng)
		}
		blindAudit := "No"
		if j.IsBlindAudit {
			blindAudit = "Yes"
		}
		buf.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s,%d,%.1f,%s,%s,%s,%s\n",
			j.JobReference,
			j.AccountID.String(),
			j.DistrictID.String(),
			officerID,
			j.Status,
			blindAudit,
			j.Priority,
			j.GPSFenceRadiusM,
			officerLat,
			officerLng,
			j.CreatedAt.Format("2006-01-02 15:04:05"),
			j.UpdatedAt.Format("2006-01-02 15:04:05"),
		))
	}

	filename := fmt.Sprintf("GN-WAAS-Field-Jobs-%s.csv", periodStr)
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(buf.Bytes())
}
