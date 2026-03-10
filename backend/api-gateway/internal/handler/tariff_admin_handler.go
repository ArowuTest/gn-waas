package handler

// TariffAdminHandler provides System Admin endpoints for managing PURC tariff rates.
//
// Q7: PURC Tariff Schedule — admin-configurable
//
// The tariff_rates table already exists and the tariff-engine already reads from it.
// This handler adds the missing admin CRUD endpoints so System Admins can:
//   - View all tariff rates (current and historical)
//   - Create new tariff rates (e.g. when PURC publishes a new schedule)
//   - Deactivate old rates (sets effective_to = today)
//   - Update rates (creates a new version, deactivates the old one)
//
// Tariff versioning strategy:
//   - Rates are versioned by effective_from date.
//   - A new PURC schedule is added as new rows with the new effective_from date.
//   - Old rows are deactivated (effective_to set, is_active = false).
//   - The tariff-engine always uses the rate with effective_from <= billing_date
//     and effective_to IS NULL (or > billing_date).
//   - This means historical bills are always recalculated with the correct rate
//     for their billing period.
//
// Access: SYSTEM_ADMIN role only (enforced by RLS + role check middleware).

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// TariffAdminHandler handles admin CRUD for tariff rates and VAT config
type TariffAdminHandler struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewTariffAdminHandler(db *pgxpool.Pool, logger *zap.Logger) *TariffAdminHandler {
	return &TariffAdminHandler{db: db, logger: logger}
}

// ── GET /api/v1/admin/tariffs ─────────────────────────────────────────────────
// Returns all tariff rates (active and historical), grouped by category.
func (h *TariffAdminHandler) ListTariffRates(c *fiber.Ctx) error {
	rows, err := h.db.Query(c.Context(), `
		SELECT id, category, tier_name,
		       min_volume_m3, max_volume_m3,
		       rate_per_m3, service_charge_ghs,
		       effective_from, effective_to,
		       COALESCE(approved_by, ''), COALESCE(regulatory_ref, ''), is_active,
		       created_at, updated_at
		FROM tariff_rates
		ORDER BY category, effective_from DESC, min_volume_m3 ASC
	`)
	if err != nil {
		h.logger.Error("ListTariffRates query failed", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch tariff rates"})
	}
	defer rows.Close()

	type tariffRow struct {
		ID               string   `json:"id"`
		Category         string   `json:"category"`
		TierName         string   `json:"tier_name"`
		MinVolumeM3      float64  `json:"min_volume_m3"`
		MaxVolumeM3      *float64 `json:"max_volume_m3"`
		RatePerM3        float64  `json:"rate_per_m3"`
		ServiceChargeGHS float64  `json:"service_charge_ghs"`
		EffectiveFrom    string   `json:"effective_from"`
		EffectiveTo      *string  `json:"effective_to"`
		ApprovedBy       string   `json:"approved_by"`
		RegulatoryRef    string   `json:"regulatory_ref"`
		IsActive         bool     `json:"is_active"`
		CreatedAt        string   `json:"created_at"`
		UpdatedAt        string   `json:"updated_at"`
	}

	var rates []tariffRow
	for rows.Next() {
		var r tariffRow
		var id uuid.UUID
		var effectiveFrom time.Time
		var effectiveTo *time.Time
		var createdAt, updatedAt time.Time

		if err := rows.Scan(
			&id, &r.Category, &r.TierName,
			&r.MinVolumeM3, &r.MaxVolumeM3,
			&r.RatePerM3, &r.ServiceChargeGHS,
			&effectiveFrom, &effectiveTo,
			&r.ApprovedBy, &r.RegulatoryRef, &r.IsActive,
			&createdAt, &updatedAt,
		); err != nil {
			continue
		}
		r.ID = id.String()
		r.EffectiveFrom = effectiveFrom.Format("2006-01-02")
		if effectiveTo != nil {
			s := effectiveTo.Format("2006-01-02")
			r.EffectiveTo = &s
		}
		r.CreatedAt = createdAt.Format(time.RFC3339)
		r.UpdatedAt = updatedAt.Format(time.RFC3339)
		rates = append(rates, r)
	}

	return c.JSON(fiber.Map{
		"tariff_rates": rates,
		"total":        len(rates),
	})
}

// ── POST /api/v1/admin/tariffs ────────────────────────────────────────────────
// Creates a new tariff rate tier.
// When PURC publishes a new schedule, create new rows with the new effective_from
// date and deactivate the old ones using PATCH /api/v1/admin/tariffs/:id/deactivate.
func (h *TariffAdminHandler) CreateTariffRate(c *fiber.Ctx) error {
	type createReq struct {
		Category         string   `json:"category"`          // RESIDENTIAL | COMMERCIAL | INDUSTRIAL | PUBLIC_GOVT | BOTTLED_WATER
		TierName         string   `json:"tier_name"`         // e.g. "Tier 1 (0-5 m³)"
		MinVolumeM3      float64  `json:"min_volume_m3"`
		MaxVolumeM3      *float64 `json:"max_volume_m3"`     // null = unlimited
		RatePerM3        float64  `json:"rate_per_m3"`
		ServiceChargeGHS float64  `json:"service_charge_ghs"`
		EffectiveFrom    string   `json:"effective_from"`    // YYYY-MM-DD
		ApprovedBy       string   `json:"approved_by"`       // e.g. "PURC-2026-01"
		RegulatoryRef    string   `json:"regulatory_ref"`    // e.g. "PURC Notice 2026/01"
	}

	var req createReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	validCategories := map[string]bool{
		"RESIDENTIAL": true, "COMMERCIAL": true, "INDUSTRIAL": true,
		"PUBLIC_GOVT": true, "BOTTLED_WATER": true,
	}
	if req.Category == "" || req.TierName == "" {
		return c.Status(400).JSON(fiber.Map{"error": "category and tier_name are required"})
	}
	if !validCategories[req.Category] {
		return c.Status(400).JSON(fiber.Map{"error": "invalid category: must be RESIDENTIAL, COMMERCIAL, INDUSTRIAL, PUBLIC_GOVT, or BOTTLED_WATER"})
	}
	if req.RatePerM3 <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "rate_per_m3 must be positive"})
	}
	if req.EffectiveFrom == "" {
		req.EffectiveFrom = time.Now().Format("2006-01-02")
	}

	effectiveFrom, err := time.Parse("2006-01-02", req.EffectiveFrom)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "effective_from must be YYYY-MM-DD"})
	}

	var newID uuid.UUID
	var createdAt time.Time
	err = h.db.QueryRow(c.Context(), `
		INSERT INTO tariff_rates (
			category, tier_name, min_volume_m3, max_volume_m3,
			rate_per_m3, service_charge_ghs, effective_from,
			approved_by, regulatory_ref, is_active
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,true)
		RETURNING id, created_at
	`,
		req.Category, req.TierName, req.MinVolumeM3, req.MaxVolumeM3,
		req.RatePerM3, req.ServiceChargeGHS, effectiveFrom,
		req.ApprovedBy, req.RegulatoryRef,
	).Scan(&newID, &createdAt)

	if err != nil {
		h.logger.Error("CreateTariffRate failed", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{"error": "failed to create tariff rate: " + err.Error()})
	}

	h.logger.Info("Tariff rate created",
		zap.String("id", newID.String()),
		zap.String("category", req.Category),
		zap.String("tier", req.TierName),
		zap.Float64("rate_per_m3", req.RatePerM3),
		zap.String("effective_from", req.EffectiveFrom),
	)

	return c.Status(201).JSON(fiber.Map{
		"id":             newID.String(),
		"category":       req.Category,
		"tier_name":      req.TierName,
		"rate_per_m3":    req.RatePerM3,
		"effective_from": req.EffectiveFrom,
		"created_at":     createdAt.Format(time.RFC3339),
		"message":        "Tariff rate created. The tariff-engine will use this rate for bills from " + req.EffectiveFrom + " onwards.",
	})
}

// ── PUT /api/v1/admin/tariffs/:id ─────────────────────────────────────────────
// Updates a tariff rate. Creates a new version and deactivates the old one.
// This preserves the audit trail — old rates are never deleted.
func (h *TariffAdminHandler) UpdateTariffRate(c *fiber.Ctx) error {
	idStr := c.Params("id")
	rateID, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid tariff rate ID"})
	}

	type updateReq struct {
		RatePerM3        *float64 `json:"rate_per_m3"`
		ServiceChargeGHS *float64 `json:"service_charge_ghs"`
		MaxVolumeM3      *float64 `json:"max_volume_m3"`
		EffectiveTo      *string  `json:"effective_to"`   // deactivate old rate
		ApprovedBy       string   `json:"approved_by"`
		RegulatoryRef    string   `json:"regulatory_ref"`
	}

	var req updateReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Build dynamic update
	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argIdx := 1

	if req.RatePerM3 != nil {
		setClauses = append(setClauses, fmt.Sprintf("rate_per_m3 = $%d", argIdx))
		args = append(args, *req.RatePerM3)
		argIdx++
	}
	if req.ServiceChargeGHS != nil {
		setClauses = append(setClauses, fmt.Sprintf("service_charge_ghs = $%d", argIdx))
		args = append(args, *req.ServiceChargeGHS)
		argIdx++
	}
	if req.MaxVolumeM3 != nil {
		setClauses = append(setClauses, fmt.Sprintf("max_volume_m3 = $%d", argIdx))
		args = append(args, *req.MaxVolumeM3)
		argIdx++
	}
	if req.EffectiveTo != nil {
		t, err := time.Parse("2006-01-02", *req.EffectiveTo)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "effective_to must be YYYY-MM-DD"})
		}
		setClauses = append(setClauses, fmt.Sprintf("effective_to = $%d", argIdx))
		args = append(args, t)
		argIdx++
		setClauses = append(setClauses, "is_active = false")
	}
	if req.ApprovedBy != "" {
		setClauses = append(setClauses, fmt.Sprintf("approved_by = $%d", argIdx))
		args = append(args, req.ApprovedBy)
		argIdx++
	}
	if req.RegulatoryRef != "" {
		setClauses = append(setClauses, fmt.Sprintf("regulatory_ref = $%d", argIdx))
		args = append(args, req.RegulatoryRef)
		argIdx++
	}

	args = append(args, rateID)
	query := fmt.Sprintf(
		"UPDATE tariff_rates SET %s WHERE id = $%d",
		joinStrings(setClauses, ", "), argIdx,
	)

	if _, err := h.db.Exec(c.Context(), query, args...); err != nil {
		h.logger.Error("UpdateTariffRate failed", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{"error": "failed to update tariff rate"})
	}

	h.logger.Info("Tariff rate updated", zap.String("id", rateID.String()))
	return c.JSON(fiber.Map{"message": "Tariff rate updated", "id": rateID.String()})
}

// ── PATCH /api/v1/admin/tariffs/:id/deactivate ────────────────────────────────
// Deactivates a tariff rate (sets effective_to = today, is_active = false).
// Use this when replacing an old PURC schedule with a new one.
func (h *TariffAdminHandler) DeactivateTariffRate(c *fiber.Ctx) error {
	idStr := c.Params("id")
	rateID, err := uuid.Parse(idStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid tariff rate ID"})
	}

	effectiveTo := time.Now()
	if dateStr := c.Query("effective_to"); dateStr != "" {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			effectiveTo = t
		}
	}

	_, err = h.db.Exec(c.Context(), `
		UPDATE tariff_rates
		SET is_active = false, effective_to = $1, updated_at = NOW()
		WHERE id = $2
	`, effectiveTo, rateID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to deactivate tariff rate"})
	}

	h.logger.Info("Tariff rate deactivated",
		zap.String("id", rateID.String()),
		zap.String("effective_to", effectiveTo.Format("2006-01-02")),
	)

	return c.JSON(fiber.Map{
		"message":      "Tariff rate deactivated",
		"id":           rateID.String(),
		"effective_to": effectiveTo.Format("2006-01-02"),
	})
}

// ── GET /api/v1/admin/tariffs/vat ─────────────────────────────────────────────
// Returns all VAT configurations (current and historical).
func (h *TariffAdminHandler) ListVATConfigs(c *fiber.Ctx) error {
	rows, err := h.db.Query(c.Context(), `
		SELECT id, rate_percentage, components, effective_from, effective_to,
		       COALESCE(regulatory_ref, ''), is_active, created_at
		FROM vat_config
		ORDER BY effective_from DESC
	`)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch VAT configs"})
	}
	defer rows.Close()

	type vatRow struct {
		ID             string      `json:"id"`
		RatePct        float64     `json:"rate_percentage"`
		Components     interface{} `json:"components"`
		EffectiveFrom  string      `json:"effective_from"`
		EffectiveTo    *string     `json:"effective_to"`
		RegulatoryRef  string      `json:"regulatory_ref"`
		IsActive       bool        `json:"is_active"`
		CreatedAt      string      `json:"created_at"`
	}

	var configs []vatRow
	for rows.Next() {
		var r vatRow
		var id uuid.UUID
		var effectiveFrom time.Time
		var effectiveTo *time.Time
		var createdAt time.Time

		if err := rows.Scan(
			&id, &r.RatePct, &r.Components,
			&effectiveFrom, &effectiveTo,
			&r.RegulatoryRef, &r.IsActive, &createdAt,
		); err != nil {
			continue
		}
		r.ID = id.String()
		r.EffectiveFrom = effectiveFrom.Format("2006-01-02")
		if effectiveTo != nil {
			s := effectiveTo.Format("2006-01-02")
			r.EffectiveTo = &s
		}
		r.CreatedAt = createdAt.Format(time.RFC3339)
		configs = append(configs, r)
	}

	return c.JSON(fiber.Map{"vat_configs": configs, "total": len(configs)})
}

// ── POST /api/v1/admin/tariffs/vat ────────────────────────────────────────────
// Creates a new VAT configuration.
func (h *TariffAdminHandler) CreateVATConfig(c *fiber.Ctx) error {
	type createReq struct {
		RatePercentage float64                `json:"rate_percentage"`
		Components     map[string]interface{} `json:"components"`
		EffectiveFrom  string                 `json:"effective_from"`
		RegulatoryRef  string                 `json:"regulatory_ref"`
	}

	var req createReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.RatePercentage <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "rate_percentage must be positive"})
	}
	if req.EffectiveFrom == "" {
		req.EffectiveFrom = time.Now().Format("2006-01-02")
	}

	effectiveFrom, err := time.Parse("2006-01-02", req.EffectiveFrom)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "effective_from must be YYYY-MM-DD"})
	}

	// Deactivate current active VAT config
	h.db.Exec(c.Context(), `
		UPDATE vat_config SET is_active = false, effective_to = $1
		WHERE is_active = true
	`, effectiveFrom)

	var newID uuid.UUID
	err = h.db.QueryRow(c.Context(), `
		INSERT INTO vat_config (rate_percentage, components, effective_from, regulatory_ref, is_active)
		VALUES ($1, $2, $3, $4, true)
		RETURNING id
	`, req.RatePercentage, req.Components, effectiveFrom, req.RegulatoryRef).Scan(&newID)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to create VAT config: " + err.Error()})
	}

	h.logger.Info("VAT config created",
		zap.String("id", newID.String()),
		zap.Float64("rate_pct", req.RatePercentage),
		zap.String("effective_from", req.EffectiveFrom),
	)

	return c.Status(201).JSON(fiber.Map{
		"id":             newID.String(),
		"rate_percentage": req.RatePercentage,
		"effective_from": req.EffectiveFrom,
		"message":        "VAT config created and activated from " + req.EffectiveFrom,
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func joinStrings(parts []string, sep string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += sep
		}
		result += p
	}
	return result
}

