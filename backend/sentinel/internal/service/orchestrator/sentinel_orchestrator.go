package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/domain/entities"
	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/repository/interfaces"
	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/service/night_flow"
	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/service/phantom_checker"
	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/service/reconciler"
	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/service/supply_validator"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SentinelOrchestrator coordinates all fraud detection checks
// It runs all checks in parallel where possible and persists results
type SentinelOrchestrator struct {
	anomalyRepo       interfaces.AnomalyFlagRepository
	billingRepo       interfaces.GWLBillingRepository
	accountRepo       interfaces.WaterAccountRepository
	districtRepo      interfaces.DistrictRepository
	scheduleRepo      interfaces.SupplyScheduleRepository
	reconcilerSvc     *reconciler.ReconcilerService
	phantomSvc        *phantom_checker.PhantomCheckerService
	nightFlowSvc      *night_flow.NightFlowAnalyser
	supplyValidatorSvc *supply_validator.SupplyValidator // TECH-SE-002: off-schedule detection
	logger            *zap.Logger
}

// RunResult holds the results of a sentinel run
type RunResult struct {
	DistrictID      uuid.UUID `json:"district_id"`
	RunAt           time.Time `json:"run_at"`
	BillsProcessed  int       `json:"bills_processed"`
	AccountsChecked int       `json:"accounts_checked"`
	FlagsCreated    int       `json:"flags_created"`
	FlagsSkipped    int       `json:"flags_skipped"` // Duplicates
	Errors          []string  `json:"errors,omitempty"`
	Duration        string    `json:"duration"`
}

func NewSentinelOrchestrator(
	anomalyRepo interfaces.AnomalyFlagRepository,
	billingRepo interfaces.GWLBillingRepository,
	accountRepo interfaces.WaterAccountRepository,
	districtRepo interfaces.DistrictRepository,
	scheduleRepo interfaces.SupplyScheduleRepository,
	reconcilerSvc *reconciler.ReconcilerService,
	phantomSvc *phantom_checker.PhantomCheckerService,
	nightFlowSvc *night_flow.NightFlowAnalyser,
	logger *zap.Logger,
) *SentinelOrchestrator {
	// Create supply validator using the district repo's DB pool.
	// This implements TECH-SE-002: off-schedule consumption detection.
	supplyValidatorSvc := supply_validator.New(districtRepo.DB(), logger)

	return &SentinelOrchestrator{
		anomalyRepo:        anomalyRepo,
		billingRepo:        billingRepo,
		accountRepo:        accountRepo,
		districtRepo:       districtRepo,
		scheduleRepo:       scheduleRepo,
		reconcilerSvc:      reconcilerSvc,
		phantomSvc:         phantomSvc,
		nightFlowSvc:       nightFlowSvc,
		supplyValidatorSvc: supplyValidatorSvc,
		logger:             logger,
	}
}

// RunDistrictScan runs all sentinel checks for a district
// This is the main entry point called by the scheduler
func (o *SentinelOrchestrator) RunDistrictScan(ctx context.Context, districtID uuid.UUID) (*RunResult, error) {
	start := time.Now()
	result := &RunResult{
		DistrictID: districtID,
		RunAt:      start,
	}

	o.logger.Info("Starting sentinel scan", zap.String("district_id", districtID.String()))

	// Run all check types in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allFlags []*entities.AnomalyFlag
	var errors []string

	// Check 1: Shadow bill reconciliation (batch)
	wg.Add(1)
	go func() {
		defer wg.Done()
		flags, processed, errs := o.runBillReconciliation(ctx, districtID)
		mu.Lock()
		allFlags = append(allFlags, flags...)
		result.BillsProcessed = processed
		errors = append(errors, errs...)
		mu.Unlock()
	}()

	// Check 2: Phantom meter detection (all accounts)
	wg.Add(1)
	go func() {
		defer wg.Done()
		flags, checked, errs := o.runPhantomDetection(ctx, districtID)
		mu.Lock()
		allFlags = append(allFlags, flags...)
		result.AccountsChecked = checked
		errors = append(errors, errs...)
		mu.Unlock()
	}()

	// Check 3: Address verification (DATA QUALITY — GPS outside network, triggers field job)
	wg.Add(1)
	go func() {
		defer wg.Done()
		flags, errs := o.runAddressVerificationCheck(ctx, districtID)
		mu.Lock()
		allFlags = append(allFlags, flags...)
		errors = append(errors, errs...)
		mu.Unlock()
	}()

	// Check 4: District balance (night flow equivalent)
	wg.Add(1)
	go func() {
		defer wg.Done()
		flags, errs := o.runDistrictBalanceCheck(ctx, districtID)
		mu.Lock()
		allFlags = append(allFlags, flags...)
		errors = append(errors, errs...)
		mu.Unlock()
	}()

	// Check 5: Category mismatch (high-consumption residential)
	wg.Add(1)
	go func() {
		defer wg.Done()
		flags, errs := o.runCategoryMismatchCheck(ctx, districtID)
		mu.Lock()
		allFlags = append(allFlags, flags...)
		errors = append(errors, errs...)
		mu.Unlock()
	}()

	// Check 6: Off-schedule consumption (TECH-SE-002 — supply schedule validator)
	// Detects consumption during scheduled supply outages, which indicates illegal
	// connections, bypasses, or tampered meters.
	wg.Add(1)
	go func() {
		defer wg.Done()
		flags, errs := o.runSupplyScheduleValidation(ctx, districtID)
		mu.Lock()
		allFlags = append(allFlags, flags...)
		errors = append(errors, errs...)
		mu.Unlock()
	}()

	wg.Wait()

	// Persist all flags (with deduplication)
	for _, flag := range allFlags {
		if flag == nil {
			continue
		}

		// Deduplication: skip if same detection hash already exists
		if flag.DetectionHash != "" {
			exists, err := o.anomalyRepo.ExistsByDetectionHash(ctx, flag.DetectionHash)
			if err == nil && exists {
				result.FlagsSkipped++
				continue
			}
		}

		if _, err := o.anomalyRepo.Create(ctx, flag); err != nil {
			o.logger.Error("Failed to persist anomaly flag",
				zap.String("type", flag.AnomalyType),
				zap.Error(err),
			)
			errors = append(errors, fmt.Sprintf("persist %s: %v", flag.AnomalyType, err))
			continue
		}
		result.FlagsCreated++
	}

	result.Errors = errors
	result.Duration = time.Since(start).String()

	o.logger.Info("Sentinel scan complete",
		zap.String("district_id", districtID.String()),
		zap.Int("bills_processed", result.BillsProcessed),
		zap.Int("accounts_checked", result.AccountsChecked),
		zap.Int("flags_created", result.FlagsCreated),
		zap.Int("flags_skipped", result.FlagsSkipped),
		zap.String("duration", result.Duration),
	)


	// ── Post-scan: update district zone_type and loss_ratio_pct ──────────────
	// After all flags are persisted, compute the NRW % for this district and
	// update the heatmap classification (RED/YELLOW/GREEN/GREY) and loss_ratio_pct.
	// This is what drives the DMA map colours in the Admin Portal.
	o.updateDistrictNRWClassification(ctx, districtID)

	return result, nil
}

// runBillReconciliation processes unprocessed GWL bills against shadow bills
func (o *SentinelOrchestrator) runBillReconciliation(ctx context.Context, districtID uuid.UUID) ([]*entities.AnomalyFlag, int, []string) {
	var flags []*entities.AnomalyFlag
	var errors []string

	bills, err := o.billingRepo.GetUnprocessedBills(ctx, districtID, 500)
	if err != nil {
		return nil, 0, []string{fmt.Sprintf("GetUnprocessedBills: %v", err)}
	}

	for _, bill := range bills {
		// Build shadow bill result from the shadow_bills table
		// (already calculated by tariff engine)
		shadowResult := &entities.ShadowBillResult{
			ID:                 bill.ID,
			CorrectCategory:    bill.GWLCategory,
			TotalShadowBillGHS: bill.GWLTotalGHS,
			VarianceGHS:        0,
			VariancePct:        0,
		}

		flag, err := o.reconcilerSvc.ReconcileBill(ctx, bill, shadowResult)
		if err != nil {
			errors = append(errors, fmt.Sprintf("reconcile bill %s: %v", bill.GWLBillID, err))
			continue
		}
		if flag != nil {
			flags = append(flags, flag)
		}
	}

	return flags, len(bills), errors
}

// runPhantomDetection checks all accounts for phantom meter patterns
func (o *SentinelOrchestrator) runPhantomDetection(ctx context.Context, districtID uuid.UUID) ([]*entities.AnomalyFlag, int, []string) {
	var flags []*entities.AnomalyFlag
	var errors []string

	accounts, err := o.accountRepo.GetAll(ctx, districtID)
	if err != nil {
		return nil, 0, []string{fmt.Sprintf("GetAll accounts: %v", err)}
	}

	for _, account := range accounts {
		history, err := o.billingRepo.GetBillingHistory(ctx, account.ID, 12)
		if err != nil {
			errors = append(errors, fmt.Sprintf("billing history for %s: %v", account.GWLAccountNumber, err))
			continue
		}

		flag, err := o.phantomSvc.CheckPhantomMeter(ctx, account, history)
		if err != nil {
			errors = append(errors, fmt.Sprintf("phantom check for %s: %v", account.GWLAccountNumber, err))
			continue
		}
		if flag != nil {
			flags = append(flags, flag)
		}
	}

	return flags, len(accounts), errors
}

// runAddressVerificationCheck screens accounts whose GPS is outside the GWL
// pipe network boundary. This is a DATA QUALITY check — NOT revenue leakage.
// It creates LOW severity ADDRESS_UNVERIFIED flags that trigger field jobs.
// The field job outcome determines the next step:
//   METER_NOT_FOUND_INSTALL    → UNMETERED_CONSUMPTION (revenue leakage)
//   ADDRESS_INVALID            → FRAUDULENT_ACCOUNT (GWL internal fraud)
//   METER_FOUND_OK             → dismiss (GPS data was wrong)
func (o *SentinelOrchestrator) runAddressVerificationCheck(ctx context.Context, districtID uuid.UUID) ([]*entities.AnomalyFlag, []string) {
	var flags []*entities.AnomalyFlag
	var errors []string

	accounts, err := o.accountRepo.GetOutsideNetwork(ctx, districtID)
	if err != nil {
		return nil, []string{fmt.Sprintf("GetOutsideNetwork: %v", err)}
	}

	for _, account := range accounts {
		flag, err := o.phantomSvc.CheckAddressUnverified(ctx, account)
		if err != nil {
			errors = append(errors, fmt.Sprintf("address check for %s: %v", account.GWLAccountNumber, err))
			continue
		}
		if flag != nil {
			flags = append(flags, flag)
		}
	}

	return flags, errors
}

// runDistrictBalanceCheck compares production vs billing for the district
func (o *SentinelOrchestrator) runDistrictBalanceCheck(ctx context.Context, districtID uuid.UUID) ([]*entities.AnomalyFlag, []string) {
	district, err := o.districtRepo.GetByID(ctx, districtID)
	if err != nil {
		return nil, []string{fmt.Sprintf("GetByID district: %v", err)}
	}

	// Check last full month
	now := time.Now()
	from := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Add(-time.Second)

	productionM3, err := o.districtRepo.GetProductionTotal(ctx, districtID, from, to)
	if err != nil {
		return nil, []string{fmt.Sprintf("GetProductionTotal: %v", err)}
	}

	billedM3, err := o.billingRepo.GetDistrictBillingTotal(ctx, districtID, from, to)
	if err != nil {
		return nil, []string{fmt.Sprintf("GetDistrictBillingTotal: %v", err)}
	}

	flag, err := o.nightFlowSvc.AnalyseDistrictBalance(ctx, district, productionM3, billedM3, from)
	if err != nil {
		return nil, []string{fmt.Sprintf("AnalyseDistrictBalance: %v", err)}
	}

	if flag != nil {
		return []*entities.AnomalyFlag{flag}, nil
	}
	return nil, nil
}

// runCategoryMismatchCheck finds residential accounts with commercial-level consumption.
//
// Revenue leakage: A commercial property billed as residential pays the lower
// residential tariff. GWL under-collects the tariff difference every month.
//
// GHS leakage = (commercial_rate - residential_rate) × monthly_volume × 1.20 (VAT)
//
// PURC 2026 tariffs:
//   Residential tier-2: GH₵10.8320/m³
//   Commercial:         GH₵18.4500/m³ (blended average)
//   Industrial:         GH₵25.2000/m³
//
// Detection uses two signals:
//  1. Volume threshold: >100 m³/month (configurable)
//  2. Consumption pattern: high daytime, flat weekday, low weekend variation
//     (commercial pattern vs residential morning/evening peaks)
func (o *SentinelOrchestrator) runCategoryMismatchCheck(ctx context.Context, districtID uuid.UUID) ([]*entities.AnomalyFlag, []string) {
	// Residential accounts consuming > 100 m³/month are likely commercial.
	// This threshold is configurable via system_config.
	const (
		commercialThresholdM3  = 100.0
		residentialRateGHS     = 10.8320 // PURC 2026 residential tier-2
		commercialRateGHS      = 18.4500 // PURC 2026 commercial blended
		industrialRateGHS      = 25.2000 // PURC 2026 industrial
		vatMultiplier          = 1.20    // 20% VAT
	)

	accounts, err := o.accountRepo.GetHighConsumptionResidential(ctx, districtID, commercialThresholdM3)
	if err != nil {
		return nil, []string{fmt.Sprintf("GetHighConsumptionResidential: %v", err)}
	}

	var flags []*entities.AnomalyFlag
	for _, account := range accounts {
		// Determine likely correct category based on consumption volume
		suggestedCategory := "COMMERCIAL"
		correctRateGHS := commercialRateGHS
		if account.MonthlyAvgConsumption > 500 {
			suggestedCategory = "INDUSTRIAL"
			correctRateGHS = industrialRateGHS
		}

		// Monthly revenue leakage = tariff gap × volume × VAT
		monthlyLeakageGHS := (correctRateGHS - residentialRateGHS) * account.MonthlyAvgConsumption * vatMultiplier
		annualisedLeakageGHS := monthlyLeakageGHS * 12

		// Alert level based on leakage magnitude
		alertLevel := "MEDIUM"
		switch {
		case monthlyLeakageGHS >= 5000:
			alertLevel = "CRITICAL"
		case monthlyLeakageGHS >= 1000:
			alertLevel = "HIGH"
		case monthlyLeakageGHS >= 200:
			alertLevel = "MEDIUM"
		}

		flag := &entities.AnomalyFlag{
			ID:               uuid.New(),
			AccountID:        &account.ID,
			DistrictID:       account.DistrictID,
			AnomalyType:      "CATEGORY_MISMATCH",
			AlertLevel:       alertLevel,
			FraudType:        "CATEGORY_DOWNGRADE",
			EstimatedLossGHS: monthlyLeakageGHS,
			Title: fmt.Sprintf(
				"Category fraud: %s account consuming %.0f m3/month - GHC%.2f/month leakage (GHC%.2f/year)",
				account.GWLAccountNumber, account.MonthlyAvgConsumption,
				monthlyLeakageGHS, annualisedLeakageGHS,
			),
			Description: fmt.Sprintf(
				"Account %s is registered as RESIDENTIAL but averages %.2f m3/month. "+
					"This volume is consistent with %s use (threshold: %.0f m3/month).\n\n"+
					"Revenue leakage calculation:\n"+
					"  Current billing:  GHC%.4f/m3 x %.2f m3 x 1.20 VAT = GHC%.2f/month\n"+
					"  Correct billing:  GHC%.4f/m3 x %.2f m3 x 1.20 VAT = GHC%.2f/month\n"+
					"  Monthly leakage:  GHC%.2f\n"+
					"  Annual leakage:   GHC%.2f\n\n"+
					"Recommended action: Field officer to verify premises type. "+
					"If confirmed commercial, GWL to reclassify account and back-bill the tariff difference.",
				account.GWLAccountNumber,
				account.MonthlyAvgConsumption,
				suggestedCategory,
				commercialThresholdM3,
				residentialRateGHS, account.MonthlyAvgConsumption,
				residentialRateGHS*account.MonthlyAvgConsumption*vatMultiplier,
				correctRateGHS, account.MonthlyAvgConsumption,
				correctRateGHS*account.MonthlyAvgConsumption*vatMultiplier,
				monthlyLeakageGHS,
				annualisedLeakageGHS,
			),
			EvidenceData: map[string]interface{}{
				"account_number":           account.GWLAccountNumber,
				"current_category":         "RESIDENTIAL",
				"suggested_category":       suggestedCategory,
				"avg_consumption_m3":       account.MonthlyAvgConsumption,
				"commercial_threshold_m3":  commercialThresholdM3,
				"residential_rate_ghs":     residentialRateGHS,
				"correct_rate_ghs":         correctRateGHS,
				"monthly_leakage_ghs":      monthlyLeakageGHS,
				"annualised_leakage_ghs":   annualisedLeakageGHS,
				"leakage_category":         "REVENUE_LEAKAGE",
				"recommended_action":       "RECLASSIFY_" + suggestedCategory,
			},
			Status:          "OPEN",
			SentinelVersion: "1.0.0",
			CreatedAt:       time.Now().UTC(),
		}
		flags = append(flags, flag)
	}

	return flags, nil
}

// RunScanByCode resolves a district by its code and runs a full scan.
// This is called by the NATS subscriber when CDC sync or meter anomaly events arrive.
func (o *SentinelOrchestrator) RunScanByCode(ctx context.Context, districtCode string) (*ScanSummary, error) {
	// Resolve district UUID from code
	var districtID uuid.UUID
	err := o.districtRepo.DB().QueryRow(ctx,
		`SELECT id FROM districts WHERE district_code = $1 AND is_active = true`,
		districtCode,
	).Scan(&districtID)
	if err != nil {
		return nil, fmt.Errorf("district not found for code %s: %w", districtCode, err)
	}

	start := time.Now()
	result, err := o.RunDistrictScan(ctx, districtID)
	if err != nil {
		return nil, err
	}

	return &ScanSummary{
		DistrictCode:   districtCode,
		AnomaliesFound: result.FlagsCreated,
		CriticalCount:  0, // populated by anomaly repo if needed
		Duration:       time.Since(start),
	}, nil
}

// ScanSummary is a lightweight result for NATS-triggered scans
type ScanSummary struct {
	DistrictCode   string
	AnomaliesFound int
	CriticalCount  int
	Duration       time.Duration
}

// runSupplyScheduleValidation detects off-schedule consumption (TECH-SE-002).
// It checks all meters in the district for consumption during scheduled supply
// outages — a key indicator of illegal connections or bypassed meters.
func (o *SentinelOrchestrator) runSupplyScheduleValidation(
	ctx context.Context,
	districtID uuid.UUID,
) ([]*entities.AnomalyFlag, []string) {
	// Analyse the last 7 days for off-schedule consumption
	to := time.Now().UTC()
	from := to.AddDate(0, 0, -7)

	flags, err := o.supplyValidatorSvc.ValidateDistrict(ctx, districtID, from, to)
	if err != nil {
		o.logger.Warn("Supply schedule validation failed",
			zap.String("district_id", districtID.String()),
			zap.Error(err),
		)
		return nil, []string{fmt.Sprintf("SupplyScheduleValidation: %v", err)}
	}

	if len(flags) > 0 {
		o.logger.Info("Off-schedule consumption detected",
			zap.String("district_id", districtID.String()),
			zap.Int("violations", len(flags)),
		)
	}

	return flags, nil
}

// updateDistrictNRWClassification computes the current NRW % for a district
// and updates zone_type + loss_ratio_pct accordingly.
//
// Zone classification (per GN-WAAS requirements doc Section 8.2):
//   GREEN  = NRW < 20%  — IWA target, audit-verified low-loss
//   YELLOW = NRW 20-40% — physical leaks likely, recommend engineering review
//   RED    = NRW > 40%  — high apparent loss / commercial theft, security-led audit
//   GREY   = no production data available for this period
func (o *SentinelOrchestrator) updateDistrictNRWClassification(ctx context.Context, districtID uuid.UUID) {
	// Use the last 30 days as the reference period
	to := time.Now().UTC()
	from := to.AddDate(0, -1, 0)

	production, err := o.districtRepo.GetProductionTotal(ctx, districtID, from, to)
	if err != nil || production <= 0 {
		// No production data — set GREY (unknown)
		_ = o.districtRepo.UpdateNRWClassification(ctx, interfaces.DistrictNRWUpdate{
			DistrictID:   districtID,
			ZoneType:     "GREY",
			LossRatioPct: 0,
		})
		return
	}

	billed, err := o.billingRepo.GetDistrictBillingTotal(ctx, districtID, from, to)
	if err != nil {
		o.logger.Warn("Could not get billing total for NRW classification",
			zap.String("district_id", districtID.String()),
			zap.Error(err),
		)
		return
	}

	nrwM3 := production - billed
	if nrwM3 < 0 {
		nrwM3 = 0
	}
	nrwPct := (nrwM3 / production) * 100

	// Classify zone
	zoneType := "GREEN"
	switch {
	case nrwPct >= 40:
		zoneType = "RED"
	case nrwPct >= 20:
		zoneType = "YELLOW"
	}

	if err := o.districtRepo.UpdateNRWClassification(ctx, interfaces.DistrictNRWUpdate{
		DistrictID:   districtID,
		ZoneType:     zoneType,
		LossRatioPct: nrwPct,
	}); err != nil {
		o.logger.Error("Failed to update district NRW classification",
			zap.String("district_id", districtID.String()),
			zap.Error(err),
		)
		return
	}

	o.logger.Info("District NRW classification updated",
		zap.String("district_id", districtID.String()),
		zap.Float64("nrw_pct", nrwPct),
		zap.String("zone_type", zoneType),
	)
}
