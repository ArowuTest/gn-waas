package seasonal

// SeasonalThresholdService adjusts NRW thresholds based on Ghana's rainy seasons.
//
// GHANA CONTEXT:
//   Ghana has two rainy seasons that significantly affect water system behaviour:
//
//   1. MAJOR RAINY SEASON (April–July):
//      - Heavy rainfall causes groundwater infiltration into pipes
//      - Apparent production increases (infiltration counted as production)
//      - Consumption drops (people use rainwater for non-potable uses)
//      - NRW % appears higher even without fraud
//      - Threshold should be RELAXED by ~5% to avoid false positives
//
//   2. MINOR RAINY SEASON (September–November):
//      - Lighter rain, moderate infiltration
//      - Threshold relaxed by ~3%
//
//   3. DRY HARMATTAN (December–March):
//      - High consumption (no rain, hot and dusty)
//      - Tighter thresholds appropriate
//      - Night-flow analysis most reliable
//
//   Without seasonal adjustment, the sentinel would generate excessive false
//   positives during rainy season, wasting field officer time and eroding
//   trust in the system.

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// SeasonalAdjustment holds the threshold adjustments for the current season
type SeasonalAdjustment struct {
	Season                   string
	Month                    int
	VarianceThresholdAdjPct  float64 // additive adjustment to base variance threshold
	NightFlowBaselineAdjM3   float64 // additional baseline night flow
	NightFlowThresholdAdjPct float64 // additive adjustment to night flow threshold
	InfiltrationFactor       float64 // fraction of production to subtract as infiltration
	ConsumptionMultiplier    float64 // expected consumption relative to dry season
}

// SeasonalThresholdService provides seasonally-adjusted thresholds
type SeasonalThresholdService struct {
	db     *pgxpool.Pool
	logger *zap.Logger
	// Cache to avoid DB hit on every sentinel run
	cache    *SeasonalAdjustment
	cacheMonth int
}

func NewSeasonalThresholdService(db *pgxpool.Pool, logger *zap.Logger) *SeasonalThresholdService {
	return &SeasonalThresholdService{db: db, logger: logger}
}

// GetCurrentAdjustment returns the seasonal adjustment for the current month.
// Falls back to hardcoded Ghana defaults if DB is unavailable.
func (s *SeasonalThresholdService) GetCurrentAdjustment(ctx context.Context) *SeasonalAdjustment {
	currentMonth := int(time.Now().Month())

	// Return cached value if same month
	if s.cache != nil && s.cacheMonth == currentMonth {
		return s.cache
	}

	adj := s.fetchFromDB(ctx, currentMonth)
	if adj == nil {
		adj = s.ghanaDefault(currentMonth)
		s.logger.Warn("Using hardcoded seasonal defaults (DB unavailable)",
			zap.Int("month", currentMonth),
			zap.String("season", adj.Season),
		)
	}

	s.cache = adj
	s.cacheMonth = currentMonth

	s.logger.Info("Seasonal threshold loaded",
		zap.String("season", adj.Season),
		zap.Int("month", currentMonth),
		zap.Float64("variance_adj_pct", adj.VarianceThresholdAdjPct),
		zap.Float64("infiltration_factor", adj.InfiltrationFactor),
	)

	return adj
}

// AdjustVarianceThreshold applies seasonal adjustment to the base variance threshold.
// Example: base=15%, major rainy adj=+5% → effective threshold=20%
func (s *SeasonalThresholdService) AdjustVarianceThreshold(
	ctx context.Context,
	baseThresholdPct float64,
) float64 {
	adj := s.GetCurrentAdjustment(ctx)
	adjusted := baseThresholdPct + adj.VarianceThresholdAdjPct
	if adjusted < 5 {
		adjusted = 5 // minimum 5% threshold
	}
	return adjusted
}

// AdjustNightFlowBaseline applies seasonal adjustment to the night flow baseline.
func (s *SeasonalThresholdService) AdjustNightFlowBaseline(
	ctx context.Context,
	baselineM3 float64,
) float64 {
	adj := s.GetCurrentAdjustment(ctx)
	return baselineM3 + adj.NightFlowBaselineAdjM3
}

// AdjustProductionForInfiltration subtracts estimated infiltration from production.
// During rainy season, some of the "production" is actually groundwater infiltration.
func (s *SeasonalThresholdService) AdjustProductionForInfiltration(
	ctx context.Context,
	productionM3 float64,
) float64 {
	adj := s.GetCurrentAdjustment(ctx)
	infiltration := productionM3 * adj.InfiltrationFactor
	adjusted := productionM3 - infiltration
	if adjusted < 0 {
		adjusted = 0
	}
	return adjusted
}

// GetSeasonName returns the current Ghana season name
func (s *SeasonalThresholdService) GetSeasonName(ctx context.Context) string {
	return s.GetCurrentAdjustment(ctx).Season
}

// ── DB FETCH ──────────────────────────────────────────────────────────────────

func (s *SeasonalThresholdService) fetchFromDB(ctx context.Context, month int) *SeasonalAdjustment {
	var adj SeasonalAdjustment
	adj.Month = month

	err := s.db.QueryRow(ctx, `
		SELECT
			season::text,
			variance_threshold_adj_pct,
			night_flow_baseline_adj_m3,
			night_flow_threshold_adj_pct,
			infiltration_factor,
			consumption_multiplier
		FROM seasonal_threshold_config
		WHERE is_active = true
		  AND (
		    (month_start <= month_end AND $1 BETWEEN month_start AND month_end)
		    OR (month_start > month_end AND ($1 >= month_start OR $1 <= month_end))
		  )
		ORDER BY effective_from DESC
		LIMIT 1
	`, month).Scan(
		&adj.Season,
		&adj.VarianceThresholdAdjPct,
		&adj.NightFlowBaselineAdjM3,
		&adj.NightFlowThresholdAdjPct,
		&adj.InfiltrationFactor,
		&adj.ConsumptionMultiplier,
	)
	if err != nil {
		return nil
	}
	return &adj
}

// ── HARDCODED GHANA DEFAULTS ──────────────────────────────────────────────────
// Used as fallback when DB is unavailable.
// Based on Ghana Meteorological Agency seasonal patterns.

func (s *SeasonalThresholdService) ghanaDefault(month int) *SeasonalAdjustment {
	switch {
	case month >= 4 && month <= 7: // April–July: Major Rainy
		return &SeasonalAdjustment{
			Season:                   "MAJOR_RAINY",
			Month:                    month,
			VarianceThresholdAdjPct:  5.0,
			NightFlowBaselineAdjM3:   2.5,
			NightFlowThresholdAdjPct: 10.0,
			InfiltrationFactor:       0.030,
			ConsumptionMultiplier:    0.90,
		}
	case month >= 9 && month <= 11: // September–November: Minor Rainy
		return &SeasonalAdjustment{
			Season:                   "MINOR_RAINY",
			Month:                    month,
			VarianceThresholdAdjPct:  3.0,
			NightFlowBaselineAdjM3:   1.5,
			NightFlowThresholdAdjPct: 5.0,
			InfiltrationFactor:       0.015,
			ConsumptionMultiplier:    0.95,
		}
	case month == 8: // August: Dry Interlude
		return &SeasonalAdjustment{
			Season:                   "DRY_INTERLUDE",
			Month:                    month,
			VarianceThresholdAdjPct:  0,
			NightFlowBaselineAdjM3:   0,
			NightFlowThresholdAdjPct: 0,
			InfiltrationFactor:       0.005,
			ConsumptionMultiplier:    1.0,
		}
	default: // December–March: Dry Harmattan
		return &SeasonalAdjustment{
			Season:                   "DRY_HARMATTAN",
			Month:                    month,
			VarianceThresholdAdjPct:  0,
			NightFlowBaselineAdjM3:   0,
			NightFlowThresholdAdjPct: 0,
			InfiltrationFactor:       0,
			ConsumptionMultiplier:    1.10,
		}
	}
}
