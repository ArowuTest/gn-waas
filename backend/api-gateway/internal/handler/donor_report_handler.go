package handler

// DonorReportHandler generates IWA/AWWA Water Balance reports for donor agencies.
//
// GHANA CONTEXT — Why donor reporting matters:
//   GN-WAAS is funded partly by World Bank, USAID, and EU grants.
//   Donors require standardised reporting in IWA/AWWA M36 format.
//   Key donor KPIs:
//   - NRW % (target: reduce from 51.6% to 20% over 5 years)
//   - Revenue Recovery Rate (GHS recovered / GHS lost)
//   - Audit Coverage (% of accounts audited per quarter)
//   - GRA Compliance Rate (% of audits with GRA-signed receipts)
//   - Field Officer Productivity (jobs completed per officer per month)
//
//   Reports are generated as JSON (for API consumers) and can be
//   downloaded as PDF via the frontend.
//
// IWA Water Balance Components:
//   System Input Volume = Authorised Consumption + Water Losses
//   Water Losses = Real Losses + Apparent Losses
//   Apparent Losses = Unauthorised Consumption + Metering Inaccuracies + Data Handling Errors
//   Real Losses = Infrastructure Leakage Index (ILI) based

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// DonorReportHandler generates donor-facing KPI reports
type DonorReportHandler struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewDonorReportHandler(db *pgxpool.Pool, logger *zap.Logger) *DonorReportHandler {
	return &DonorReportHandler{db: db, logger: logger}
}

// ── GET /api/v1/reports/donor/kpis ───────────────────────────────────────────
// Returns current donor KPI snapshot

func (h *DonorReportHandler) GetKPIs(c *fiber.Ctx) error {
	periodStr := c.Query("period") // YYYY-MM, defaults to current month
	districtCode := c.Query("district_code")

	var periodStart, periodEnd time.Time
	if periodStr != "" {
		t, err := time.Parse("2006-01", periodStr)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid period format, use YYYY-MM"})
		}
		periodStart = t
		periodEnd = t.AddDate(0, 1, -1)
	} else {
		now := time.Now()
		periodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		periodEnd = periodStart.AddDate(0, 1, -1)
	}

	// Resolve district filter
	districtFilter := ""
	districtArgs := []interface{}{periodStart, periodEnd}
	if districtCode != "" {
		districtFilter = " AND d.district_code = $3"
		districtArgs = append(districtArgs, districtCode)
	}

	// ── IWA Water Balance ─────────────────────────────────────────────────────
	var waterBalance struct {
		SystemInputVolumeM3     float64 `json:"system_input_volume_m3"`
		AuthorisedConsumptionM3 float64 `json:"authorised_consumption_m3"`
		BilledAuthorisedM3      float64 `json:"billed_authorised_m3"`
		UnbilledAuthorisedM3    float64 `json:"unbilled_authorised_m3"`
		WaterLossesM3           float64 `json:"water_losses_m3"`
		ApparentLossesM3        float64 `json:"apparent_losses_m3"`
		RealLossesM3            float64 `json:"real_losses_m3"`
		NRWPercent              float64 `json:"nrw_percent"`
		NRWTargetPercent        float64 `json:"nrw_target_percent"`
		ILI                     float64 `json:"infrastructure_leakage_index"`
	}

	h.db.QueryRow(c.Context(), fmt.Sprintf(`
		SELECT
			COALESCE(SUM(dpr.total_production_m3), 0) AS system_input,
			COALESCE(SUM(dpr.billed_consumption_m3), 0) AS billed_auth,
			COALESCE(SUM(dpr.unbilled_authorised_m3), 0) AS unbilled_auth,
			COALESCE(SUM(dpr.apparent_losses_m3), 0) AS apparent,
			COALESCE(SUM(dpr.real_losses_m3), 0) AS real_losses
		FROM district_production_records dpr
		JOIN districts d ON d.id = dpr.district_id
		WHERE dpr.record_date BETWEEN $1 AND $2 %s
	`, districtFilter), districtArgs...).Scan(
		&waterBalance.SystemInputVolumeM3,
		&waterBalance.BilledAuthorisedM3,
		&waterBalance.UnbilledAuthorisedM3,
		&waterBalance.ApparentLossesM3,
		&waterBalance.RealLossesM3,
	)

	waterBalance.AuthorisedConsumptionM3 = waterBalance.BilledAuthorisedM3 + waterBalance.UnbilledAuthorisedM3
	waterBalance.WaterLossesM3 = waterBalance.ApparentLossesM3 + waterBalance.RealLossesM3
	if waterBalance.SystemInputVolumeM3 > 0 {
		waterBalance.NRWPercent = (waterBalance.WaterLossesM3 / waterBalance.SystemInputVolumeM3) * 100
	}
	waterBalance.NRWTargetPercent = 20.0

	// ILI = Real Losses / Unavoidable Annual Real Losses
	// Simplified: ILI = (Real Losses / System Input) / 0.05 (5% benchmark)
	if waterBalance.SystemInputVolumeM3 > 0 {
		realLossFraction := waterBalance.RealLossesM3 / waterBalance.SystemInputVolumeM3
		waterBalance.ILI = realLossFraction / 0.05
	}

	// ── Revenue KPIs ──────────────────────────────────────────────────────────
	var revenueKPIs struct {
		TotalBilledGHS          float64 `json:"total_billed_ghs"`
		TotalCollectedGHS       float64 `json:"total_collected_ghs"`
		CollectionEfficiencyPct float64 `json:"collection_efficiency_pct"`
		RevenueGapGHS           float64 `json:"revenue_gap_ghs"`
		RecoveredGHS            float64 `json:"recovered_ghs"`
		RecoveryRatePct         float64 `json:"recovery_rate_pct"`
		SuccessFeesGHS          float64 `json:"success_fees_ghs"` // 3% of recovered
		GHSUSDRate              float64 `json:"ghs_usd_rate"`
		RecoveredUSD            float64 `json:"recovered_usd"`
	}

	h.db.QueryRow(c.Context(), fmt.Sprintf(`
		SELECT
			COALESCE(SUM(dpr.total_billed_ghs), 0),
			COALESCE(SUM(dpr.total_collected_ghs), 0),
			COALESCE(SUM(dpr.revenue_gap_ghs), 0)
		FROM district_production_records dpr
		JOIN districts d ON d.id = dpr.district_id
		WHERE dpr.record_date BETWEEN $1 AND $2 %s
	`, districtFilter), districtArgs...).Scan(
		&revenueKPIs.TotalBilledGHS,
		&revenueKPIs.TotalCollectedGHS,
		&revenueKPIs.RevenueGapGHS,
	)

	if revenueKPIs.TotalBilledGHS > 0 {
		revenueKPIs.CollectionEfficiencyPct = (revenueKPIs.TotalCollectedGHS / revenueKPIs.TotalBilledGHS) * 100
	}

	// Recovery from revenue_recovery_events
	h.db.QueryRow(c.Context(), `
		SELECT COALESCE(SUM(recovered_ghs), 0)
		FROM revenue_recovery_events
		WHERE COALESCE(confirmed_at, created_at) BETWEEN $1 AND $2
		  AND status = 'CONFIRMED'
	`, periodStart, periodEnd).Scan(&revenueKPIs.RecoveredGHS)

	if revenueKPIs.RevenueGapGHS > 0 {
		revenueKPIs.RecoveryRatePct = (revenueKPIs.RecoveredGHS / revenueKPIs.RevenueGapGHS) * 100
	}
	revenueKPIs.SuccessFeesGHS = revenueKPIs.RecoveredGHS * 0.03

	// Current GHS/USD rate
	h.db.QueryRow(c.Context(), `
		SELECT ghs_per_usd FROM exchange_rates
		WHERE currency_pair = 'GHS/USD'
		ORDER BY effective_date DESC LIMIT 1
	`).Scan(&revenueKPIs.GHSUSDRate)
	if revenueKPIs.GHSUSDRate > 0 {
		revenueKPIs.RecoveredUSD = revenueKPIs.RecoveredGHS / revenueKPIs.GHSUSDRate
	}

	// ── Audit KPIs ────────────────────────────────────────────────────────────
	var auditKPIs struct {
		TotalAudits          int     `json:"total_audits"`
		CompletedAudits      int     `json:"completed_audits"`
		GRASignedAudits      int     `json:"gra_signed_audits"`
		ProvisionalAudits    int     `json:"provisional_audits"`
		GRAComplianceRatePct float64 `json:"gra_compliance_rate_pct"`
		AnomaliesDetected    int     `json:"anomalies_detected"`
		AnomaliesConfirmed   int     `json:"anomalies_confirmed"`
		ConfirmationRatePct  float64 `json:"confirmation_rate_pct"`
	}

	h.db.QueryRow(c.Context(), `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'COMPLETED'),
			COUNT(*) FILTER (WHERE gra_status = 'SIGNED'),
			COUNT(*) FILTER (WHERE gra_status = 'PROVISIONAL')
		FROM audit_events
		WHERE created_at BETWEEN $1 AND $2
	`, periodStart, periodEnd).Scan(
		&auditKPIs.TotalAudits,
		&auditKPIs.CompletedAudits,
		&auditKPIs.GRASignedAudits,
		&auditKPIs.ProvisionalAudits,
	)

	if auditKPIs.CompletedAudits > 0 {
		auditKPIs.GRAComplianceRatePct = float64(auditKPIs.GRASignedAudits) / float64(auditKPIs.CompletedAudits) * 100
	}

	h.db.QueryRow(c.Context(), `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'CONFIRMED')
		FROM anomaly_flags
		WHERE detected_at BETWEEN $1 AND $2
	`, periodStart, periodEnd).Scan(
		&auditKPIs.AnomaliesDetected,
		&auditKPIs.AnomaliesConfirmed,
	)

	if auditKPIs.AnomaliesDetected > 0 {
		auditKPIs.ConfirmationRatePct = float64(auditKPIs.AnomaliesConfirmed) / float64(auditKPIs.AnomaliesDetected) * 100
	}

	// ── Field Operations KPIs ─────────────────────────────────────────────────
	var fieldKPIs struct {
		TotalJobs            int     `json:"total_jobs"`
		CompletedJobs        int     `json:"completed_jobs"`
		CompletionRatePct    float64 `json:"completion_rate_pct"`
		ActiveOfficers       int     `json:"active_officers"`
		JobsPerOfficer       float64 `json:"jobs_per_officer"`
		AvgJobDurationHours  float64 `json:"avg_job_duration_hours"`
		GPSConfirmedAccounts int     `json:"gps_confirmed_accounts"`
	}

	h.db.QueryRow(c.Context(), `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'COMPLETED'),
			COUNT(DISTINCT assigned_to) FILTER (WHERE status = 'COMPLETED')
		FROM field_jobs
		WHERE created_at BETWEEN $1 AND $2
	`, periodStart, periodEnd).Scan(
		&fieldKPIs.TotalJobs,
		&fieldKPIs.CompletedJobs,
		&fieldKPIs.ActiveOfficers,
	)

	if fieldKPIs.TotalJobs > 0 {
		fieldKPIs.CompletionRatePct = float64(fieldKPIs.CompletedJobs) / float64(fieldKPIs.TotalJobs) * 100
	}
	if fieldKPIs.ActiveOfficers > 0 {
		fieldKPIs.JobsPerOfficer = float64(fieldKPIs.CompletedJobs) / float64(fieldKPIs.ActiveOfficers)
	}

	h.db.QueryRow(c.Context(), `
		SELECT COUNT(*) FROM water_accounts
		WHERE gps_source = 'FIELD_CONFIRMED'
	`).Scan(&fieldKPIs.GPSConfirmedAccounts)

	// ── MoMo Reconciliation KPIs ─────────────────────────────────────────────
	// NOTE: GN-WAAS does not process MoMo payments directly.
	// These KPIs are populated only if GWL provides a payment data feed.
	var momoKPIs struct {
		TotalTransactions    int     `json:"total_transactions"`
		MatchedTransactions  int     `json:"matched_transactions"`
		GhostAccounts        int     `json:"ghost_accounts"`
		FraudFlags           int     `json:"fraud_flags"`
		TotalAmountGHS       float64 `json:"total_amount_ghs"`
		UnmatchedAmountGHS   float64 `json:"unmatched_amount_ghs"`
	}
	// Zeroed — MoMo data not ingested by GN-WAAS (GWL owns payment reconciliation)

	// ── Whistleblower KPIs ────────────────────────────────────────────────────
	var whistleblowerKPIs struct {
		TotalTips          int     `json:"total_tips"`
		InvestigatedTips   int     `json:"investigated_tips"`
		ConfirmedFraud     int     `json:"confirmed_fraud"`
		RewardsIssuedGHS   float64 `json:"rewards_issued_ghs"`
	}

	h.db.QueryRow(c.Context(), `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status IN ('INVESTIGATING','CONFIRMED','REWARDED')),
			COUNT(*) FILTER (WHERE status = 'CONFIRMED'),
			COALESCE(SUM(reward_amount_ghs) FILTER (WHERE status = 'REWARDED'), 0)
		FROM whistleblower_tips
		WHERE created_at BETWEEN $1 AND $2
	`, periodStart, periodEnd).Scan(
		&whistleblowerKPIs.TotalTips,
		&whistleblowerKPIs.InvestigatedTips,
		&whistleblowerKPIs.ConfirmedFraud,
		&whistleblowerKPIs.RewardsIssuedGHS,
	)

	// ── Snapshot to DB ────────────────────────────────────────────────────────
	h.saveKPISnapshot(c, periodStart, waterBalance.NRWPercent,
		revenueKPIs.RecoveredGHS, auditKPIs.GRAComplianceRatePct,
		fieldKPIs.CompletionRatePct, auditKPIs.TotalAudits)

	return c.JSON(fiber.Map{
		"period":          fmt.Sprintf("%s to %s", periodStart.Format("2006-01-02"), periodEnd.Format("2006-01-02")),
		"generated_at":    time.Now().UTC().Format(time.RFC3339),
		"water_balance":   waterBalance,
		"revenue":         revenueKPIs,
		"audits":          auditKPIs,
		"field_ops":       fieldKPIs,
		"momo":            momoKPIs,
		"whistleblower":   whistleblowerKPIs,
		"standard":        "IWA/AWWA M36 Water Balance Framework",
		"reporting_entity": "Ghana National Water Audit & Assurance System (GN-WAAS)",
	})
}

// ── GET /api/v1/reports/donor/trend ──────────────────────────────────────────
// Returns 12-month trend data for donor presentations

func (h *DonorReportHandler) GetTrend(c *fiber.Ctx) error {
	months := c.QueryInt("months", 12)
	if months > 24 {
		months = 24
	}

	type monthlyPoint struct {
		Month           string  `json:"month"`
		NRWPercent      float64 `json:"nrw_percent"`
		RecoveredGHS    float64 `json:"recovered_ghs"`
		AuditsCompleted int     `json:"audits_completed"`
		GRASignedPct    float64 `json:"gra_signed_pct"`
	}

	var trend []monthlyPoint
	now := time.Now()

	for i := months - 1; i >= 0; i-- {
		t := now.AddDate(0, -i, 0)
		start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, -1)

		var pt monthlyPoint
		pt.Month = start.Format("2006-01")

		// NRW
		var input, losses float64
		h.db.QueryRow(c.Context(), `
			SELECT
				COALESCE(SUM(total_production_m3), 0),
				COALESCE(SUM(apparent_losses_m3 + real_losses_m3), 0)
			FROM district_production_records
			WHERE record_date BETWEEN $1 AND $2
		`, start, end).Scan(&input, &losses)
		if input > 0 {
			pt.NRWPercent = (losses / input) * 100
		}

		// Recovery
		h.db.QueryRow(c.Context(), `
			SELECT COALESCE(SUM(recovered_ghs), 0)
			FROM revenue_recovery_events
			WHERE COALESCE(confirmed_at, created_at) BETWEEN $1 AND $2 AND status = 'CONFIRMED'
		`, start, end).Scan(&pt.RecoveredGHS)

		// Audits
		var total, signed int
		h.db.QueryRow(c.Context(), `
			SELECT
				COUNT(*) FILTER (WHERE status = 'COMPLETED'),
				COUNT(*) FILTER (WHERE gra_status = 'SIGNED')
			FROM audit_events
			WHERE created_at BETWEEN $1 AND $2
		`, start, end).Scan(&total, &signed)
		pt.AuditsCompleted = total
		if total > 0 {
			pt.GRASignedPct = float64(signed) / float64(total) * 100
		}

		trend = append(trend, pt)
	}

	return c.JSON(fiber.Map{
		"trend":        trend,
		"months":       months,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// saveKPISnapshot persists a KPI snapshot for historical tracking
func (h *DonorReportHandler) saveKPISnapshot(
	c *fiber.Ctx,
	period time.Time,
	nrwPct, recoveredGHS, graCompliancePct, fieldCompletionPct float64,
	totalAudits int,
) {
	h.db.Exec(c.Context(), `
		INSERT INTO donor_kpi_snapshots (
			snapshot_date, period_start, period_end,
			nrw_percent, revenue_recovered_ghs,
			gra_compliance_rate_pct, field_completion_rate_pct,
			total_audits_completed
		) VALUES (NOW(), $1, $1 + INTERVAL '1 month', $2, $3, $4, $5, $6)
		ON CONFLICT (period_start) DO UPDATE SET
			nrw_percent = EXCLUDED.nrw_percent,
			revenue_recovered_ghs = EXCLUDED.revenue_recovered_ghs,
			gra_compliance_rate_pct = EXCLUDED.gra_compliance_rate_pct,
			field_completion_rate_pct = EXCLUDED.field_completion_rate_pct,
			total_audits_completed = EXCLUDED.total_audits_completed,
			updated_at = NOW()
	`, period, nrwPct, recoveredGHS, graCompliancePct, fieldCompletionPct, totalAudits)
}
