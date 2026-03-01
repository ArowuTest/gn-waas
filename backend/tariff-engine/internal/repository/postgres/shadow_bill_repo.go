package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/ArowuTest/gn-waas/backend/tariff-engine/internal/domain/entities"
	"github.com/ArowuTest/gn-waas/backend/tariff-engine/internal/repository/interfaces"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ShadowBillRepository implements interfaces.ShadowBillRepository using PostgreSQL
type ShadowBillRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewShadowBillRepository(db *pgxpool.Pool, logger *zap.Logger) *ShadowBillRepository {
	return &ShadowBillRepository{db: db, logger: logger}
}

// Create persists a shadow bill calculation
func (r *ShadowBillRepository) Create(
	ctx context.Context,
	bill *entities.ShadowBillCalculation,
) error {

	query := `
		INSERT INTO shadow_bills (
			gwl_bill_id, account_id, billing_period_start, billing_period_end,
			consumption_m3, correct_category, tariff_rate_id, vat_config_id,
			tier1_volume_m3, tier1_rate, tier1_amount_ghs,
			tier2_volume_m3, tier2_rate, tier2_amount_ghs,
			service_charge_ghs, subtotal_ghs, vat_amount_ghs,
			total_shadow_bill_ghs, gwl_total_ghs,
			variance_ghs, variance_pct,
			is_flagged, flag_reason,
			calculated_at, calculation_version
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19,
			$20, $21, $22, $23, $24, $25
		)
		ON CONFLICT (gwl_bill_id) DO UPDATE SET
			correct_category = EXCLUDED.correct_category,
			total_shadow_bill_ghs = EXCLUDED.total_shadow_bill_ghs,
			variance_ghs = EXCLUDED.variance_ghs,
			variance_pct = EXCLUDED.variance_pct,
			is_flagged = EXCLUDED.is_flagged,
			flag_reason = EXCLUDED.flag_reason,
			calculated_at = EXCLUDED.calculated_at,
			calculation_version = EXCLUDED.calculation_version`

	_, err := r.db.Exec(ctx, query,
		bill.GWLBillID, bill.AccountID, nil, nil, // period dates fetched from gwl_billing_records
		bill.ConsumptionM3, bill.CorrectCategory, bill.TariffRateID, bill.VATConfigID,
		bill.Tier1VolumeM3, bill.Tier1Rate, bill.Tier1AmountGHS,
		bill.Tier2VolumeM3, bill.Tier2Rate, bill.Tier2AmountGHS,
		bill.ServiceChargeGHS, bill.SubtotalGHS, bill.VATAmountGHS,
		bill.TotalShadowBillGHS, bill.GWLTotalGHS,
		bill.VarianceGHS, bill.VariancePct,
		bill.IsFlagged, bill.FlagReason,
		bill.CalculatedAt, bill.CalculationVersion,
	)

	if err != nil {
		return fmt.Errorf("Create shadow bill failed: %w", err)
	}

	return nil
}

// GetByGWLBillID returns the shadow bill for a GWL bill
func (r *ShadowBillRepository) GetByGWLBillID(
	ctx context.Context,
	gwlBillID uuid.UUID,
) (*entities.ShadowBillCalculation, error) {

	query := `
		SELECT
			id, gwl_bill_id, account_id,
			consumption_m3, correct_category,
			tariff_rate_id, vat_config_id,
			tier1_volume_m3, tier1_rate, tier1_amount_ghs,
			tier2_volume_m3, tier2_rate, tier2_amount_ghs,
			service_charge_ghs, subtotal_ghs, vat_amount_ghs,
			total_shadow_bill_ghs, gwl_total_ghs,
			variance_ghs, variance_pct,
			is_flagged, flag_reason,
			calculated_at, calculation_version
		FROM shadow_bills
		WHERE gwl_bill_id = $1`

	row := r.db.QueryRow(ctx, query, gwlBillID)
	bill := &entities.ShadowBillCalculation{}

	err := row.Scan(
		nil, // id - not in struct
		&bill.GWLBillID, &bill.AccountID,
		&bill.ConsumptionM3, &bill.CorrectCategory,
		&bill.TariffRateID, &bill.VATConfigID,
		&bill.Tier1VolumeM3, &bill.Tier1Rate, &bill.Tier1AmountGHS,
		&bill.Tier2VolumeM3, &bill.Tier2Rate, &bill.Tier2AmountGHS,
		&bill.ServiceChargeGHS, &bill.SubtotalGHS, &bill.VATAmountGHS,
		&bill.TotalShadowBillGHS, &bill.GWLTotalGHS,
		&bill.VarianceGHS, &bill.VariancePct,
		&bill.IsFlagged, &bill.FlagReason,
		&bill.CalculatedAt, &bill.CalculationVersion,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("shadow bill for GWL bill %s not found", gwlBillID)
		}
		return nil, fmt.Errorf("GetByGWLBillID failed: %w", err)
	}

	return bill, nil
}

// GetFlaggedBills returns all flagged shadow bills for a district/period
func (r *ShadowBillRepository) GetFlaggedBills(
	ctx context.Context,
	districtID uuid.UUID,
	from, to time.Time,
) ([]*entities.ShadowBillCalculation, error) {

	query := `
		SELECT
			sb.gwl_bill_id, sb.account_id,
			sb.consumption_m3, sb.correct_category,
			sb.tariff_rate_id, sb.vat_config_id,
			sb.tier1_volume_m3, sb.tier1_rate, sb.tier1_amount_ghs,
			sb.tier2_volume_m3, sb.tier2_rate, sb.tier2_amount_ghs,
			sb.service_charge_ghs, sb.subtotal_ghs, sb.vat_amount_ghs,
			sb.total_shadow_bill_ghs, sb.gwl_total_ghs,
			sb.variance_ghs, sb.variance_pct,
			sb.is_flagged, sb.flag_reason,
			sb.calculated_at, sb.calculation_version
		FROM shadow_bills sb
		JOIN gwl_billing_records gbr ON sb.gwl_bill_id = gbr.id
		JOIN water_accounts wa ON sb.account_id = wa.id
		WHERE wa.district_id = $1
		  AND sb.is_flagged = TRUE
		  AND gbr.billing_period_start >= $2
		  AND gbr.billing_period_end <= $3
		ORDER BY ABS(sb.variance_pct) DESC`

	rows, err := r.db.Query(ctx, query, districtID, from, to)
	if err != nil {
		return nil, fmt.Errorf("GetFlaggedBills failed: %w", err)
	}
	defer rows.Close()

	var bills []*entities.ShadowBillCalculation
	for rows.Next() {
		bill := &entities.ShadowBillCalculation{}
		err := rows.Scan(
			&bill.GWLBillID, &bill.AccountID,
			&bill.ConsumptionM3, &bill.CorrectCategory,
			&bill.TariffRateID, &bill.VATConfigID,
			&bill.Tier1VolumeM3, &bill.Tier1Rate, &bill.Tier1AmountGHS,
			&bill.Tier2VolumeM3, &bill.Tier2Rate, &bill.Tier2AmountGHS,
			&bill.ServiceChargeGHS, &bill.SubtotalGHS, &bill.VATAmountGHS,
			&bill.TotalShadowBillGHS, &bill.GWLTotalGHS,
			&bill.VarianceGHS, &bill.VariancePct,
			&bill.IsFlagged, &bill.FlagReason,
			&bill.CalculatedAt, &bill.CalculationVersion,
		)
		if err != nil {
			return nil, fmt.Errorf("scan shadow bill failed: %w", err)
		}
		bills = append(bills, bill)
	}

	return bills, rows.Err()
}

// GetVarianceSummary returns variance statistics for a district
func (r *ShadowBillRepository) GetVarianceSummary(
	ctx context.Context,
	districtID uuid.UUID,
	from, to time.Time,
) (*interfaces.VarianceSummary, error) {

	query := `
		SELECT
			COUNT(*) AS total_bills,
			COUNT(*) FILTER (WHERE sb.is_flagged = TRUE) AS flagged_bills,
			COALESCE(SUM(sb.variance_ghs), 0) AS total_variance_ghs,
			COALESCE(AVG(ABS(sb.variance_pct)), 0) AS avg_variance_pct,
			COALESCE(MAX(ABS(sb.variance_pct)), 0) AS max_variance_pct,
			COALESCE(SUM(sb.variance_ghs) FILTER (WHERE sb.variance_ghs > 0), 0) AS estimated_loss_ghs
		FROM shadow_bills sb
		JOIN gwl_billing_records gbr ON sb.gwl_bill_id = gbr.id
		JOIN water_accounts wa ON sb.account_id = wa.id
		WHERE wa.district_id = $1
		  AND gbr.billing_period_start >= $2
		  AND gbr.billing_period_end <= $3`

	summary := &interfaces.VarianceSummary{DistrictID: districtID}

	err := r.db.QueryRow(ctx, query, districtID, from, to).Scan(
		&summary.TotalBills,
		&summary.FlaggedBills,
		&summary.TotalVarianceGHS,
		&summary.AvgVariancePct,
		&summary.MaxVariancePct,
		&summary.EstimatedLossGHS,
	)
	if err != nil {
		return nil, fmt.Errorf("GetVarianceSummary failed: %w", err)
	}

	return summary, nil
}
