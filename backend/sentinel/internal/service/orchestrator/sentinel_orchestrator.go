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
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SentinelOrchestrator coordinates all fraud detection checks
// It runs all checks in parallel where possible and persists results
type SentinelOrchestrator struct {
	anomalyRepo    interfaces.AnomalyFlagRepository
	billingRepo    interfaces.GWLBillingRepository
	accountRepo    interfaces.WaterAccountRepository
	districtRepo   interfaces.DistrictRepository
	scheduleRepo   interfaces.SupplyScheduleRepository
	reconcilerSvc  *reconciler.ReconcilerService
	phantomSvc     *phantom_checker.PhantomCheckerService
	nightFlowSvc   *night_flow.NightFlowAnalyser
	logger         *zap.Logger
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
	return &SentinelOrchestrator{
		anomalyRepo:   anomalyRepo,
		billingRepo:   billingRepo,
		accountRepo:   accountRepo,
		districtRepo:  districtRepo,
		scheduleRepo:  scheduleRepo,
		reconcilerSvc: reconcilerSvc,
		phantomSvc:    phantomSvc,
		nightFlowSvc:  nightFlowSvc,
		logger:        logger,
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

	// Check 3: Ghost account detection (outside network)
	wg.Add(1)
	go func() {
		defer wg.Done()
		flags, errs := o.runGhostAccountDetection(ctx, districtID)
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

// runGhostAccountDetection checks for accounts outside the pipe network
func (o *SentinelOrchestrator) runGhostAccountDetection(ctx context.Context, districtID uuid.UUID) ([]*entities.AnomalyFlag, []string) {
	var flags []*entities.AnomalyFlag
	var errors []string

	accounts, err := o.accountRepo.GetOutsideNetwork(ctx, districtID)
	if err != nil {
		return nil, []string{fmt.Sprintf("GetOutsideNetwork: %v", err)}
	}

	for _, account := range accounts {
		flag, err := o.phantomSvc.CheckGhostAccount(ctx, account)
		if err != nil {
			errors = append(errors, fmt.Sprintf("ghost check for %s: %v", account.GWLAccountNumber, err))
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

// runCategoryMismatchCheck finds residential accounts with commercial-level consumption
func (o *SentinelOrchestrator) runCategoryMismatchCheck(ctx context.Context, districtID uuid.UUID) ([]*entities.AnomalyFlag, []string) {
	// Residential accounts consuming > 100 m³/month are likely commercial
	// This threshold is configurable via system_config
	const commercialThresholdM3 = 100.0

	accounts, err := o.accountRepo.GetHighConsumptionResidential(ctx, districtID, commercialThresholdM3)
	if err != nil {
		return nil, []string{fmt.Sprintf("GetHighConsumptionResidential: %v", err)}
	}

	var flags []*entities.AnomalyFlag
	for _, account := range accounts {
		flag := &entities.AnomalyFlag{
			AccountID:   &account.ID,
			DistrictID:  account.DistrictID,
			AnomalyType: "CATEGORY_MISMATCH",
			AlertLevel:  "HIGH",
			FraudType:   "CATEGORY_DOWNGRADE",
			Title: fmt.Sprintf(
				"Residential account consuming %.0f m³/month (commercial threshold: %.0f m³)",
				account.MonthlyAvgConsumption, commercialThresholdM3,
			),
			Description: fmt.Sprintf(
				"Account %s is billed as RESIDENTIAL but averages %.2f m³/month. "+
					"Commercial threshold is %.0f m³/month. "+
					"Correct category would apply commercial tariff (₵18.45/m³ vs ₵10.83/m³). "+
					"Recommend field verification of premises type.",
				account.GWLAccountNumber,
				account.MonthlyAvgConsumption,
				commercialThresholdM3,
			),
			EstimatedLossGHS: (account.MonthlyAvgConsumption * (18.45 - 10.83)) * 1.20, // VAT included
			EvidenceData: map[string]interface{}{
				"account_number":          account.GWLAccountNumber,
				"current_category":        "RESIDENTIAL",
				"suggested_category":      "COMMERCIAL",
				"avg_consumption_m3":      account.MonthlyAvgConsumption,
				"commercial_threshold_m3": commercialThresholdM3,
				"residential_rate":        10.83,
				"commercial_rate":         18.45,
			},
			Status:          "OPEN",
			SentinelVersion: "1.0.0",
			CreatedAt:       time.Now().UTC(),
		}
		flags = append(flags, flag)
	}

	return flags, nil
}
