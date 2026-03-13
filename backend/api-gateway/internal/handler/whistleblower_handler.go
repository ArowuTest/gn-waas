package handler

// WhistleblowerHandler handles anonymous tip submissions.
//
// GHANA CONTEXT — Why this matters:
//   Personnel collusion is explicitly identified as a key fraud driver in GN-WAAS.
//   GWL staff manipulate bills, create ghost accounts, and accept bribes.
//   A whistleblower system allows:
//   1. Customers to report suspicious billing
//   2. GWL staff to report colleagues without fear of retaliation
//   3. Community members to report illegal connections
//
//   Key design decisions for Ghana:
//   - NO LOGIN REQUIRED: Tipsters must remain anonymous
//   - SMS receipt: Tipster gets a reference number via SMS (optional)
//   - Reward system: 3% of recovered amount if tip leads to confirmed fraud
//   - Twi/English: Tips can be submitted in local language
//   - IP hash only: We store SHA-256 of IP, not the IP itself
//
// Public endpoint: POST /api/v1/tips (no authentication)
// Admin endpoints: GET/PATCH /api/v1/admin/tips (SYSTEM_ADMIN only)

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// WhistleblowerHandler handles tip submissions and management
type WhistleblowerHandler struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewWhistleblowerHandler(db *pgxpool.Pool, logger *zap.Logger) *WhistleblowerHandler {
	return &WhistleblowerHandler{db: db, logger: logger}
}

// ── POST /api/v1/tips (PUBLIC — no auth) ─────────────────────────────────────
// Submit an anonymous tip. Returns a reference number for tracking.

func (h *WhistleblowerHandler) SubmitTip(c *fiber.Ctx) error {
	type submitReq struct {
		Category          string   `json:"category"`           // tip_category enum
		DistrictCode      string   `json:"district_code"`      // optional
		GWLAccountNumber  string   `json:"gwl_account_number"` // optional
		Description       string   `json:"description"`        // required
		PhotoURLs         []string `json:"photo_urls"`         // optional
		LocationLat       *float64 `json:"location_lat"`       // optional
		LocationLng       *float64 `json:"location_lng"`       // optional
		ContactPhone      string   `json:"contact_phone"`      // optional (for reward)
		ContactMethod     string   `json:"contact_method"`     // 'SMS', 'PHONE', 'NONE'
		Language          string   `json:"language"`           // 'en', 'tw', 'ga', 'ee', 'ha'
	}

	var req submitReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Validate
	if strings.TrimSpace(req.Description) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "description is required"})
	}
	if len(req.Description) < 20 {
		return c.Status(400).JSON(fiber.Map{"error": "description must be at least 20 characters"})
	}

	validCategories := map[string]bool{
		"GHOST_ACCOUNT": true, "PHANTOM_METER": true, "BILLING_MANIPULATION": true,
		"METER_TAMPERING": true, "COLLUSION": true, "ILLEGAL_CONNECTION": true,
		"FIELD_OFFICER_FRAUD": true, "OTHER": true,
	}
	if req.Category == "" {
		req.Category = "OTHER"
	}
	if !validCategories[req.Category] {
		return c.Status(400).JSON(fiber.Map{"error": "invalid category"})
	}

	// Generate tip reference: TIP-YYYY-XXXXXX
	tipRef := fmt.Sprintf("TIP-%d-%06d",
		time.Now().Year(),
		time.Now().UnixNano()%1000000,
	)

	// Hash IP for privacy (store hash, not IP)
	clientIP := c.IP()
	ipHash := fmt.Sprintf("%x", sha256.Sum256([]byte(clientIP)))
	uaHash := fmt.Sprintf("%x", sha256.Sum256([]byte(c.Get("User-Agent"))))

	// Resolve district_id from district_code
	var districtID *uuid.UUID
	if req.DistrictCode != "" {
		var dID uuid.UUID
		if err := h.db.QueryRow(c.Context(),
			`SELECT id FROM districts WHERE district_code = $1`, req.DistrictCode,
		).Scan(&dID); err == nil {
			districtID = &dID
		}
	}

	// Determine if reward-eligible (contact provided)
	rewardEligible := req.ContactPhone != "" && req.ContactMethod != "NONE"

	// Insert tip
	var tipID uuid.UUID
	err := h.db.QueryRow(c.Context(), `
		INSERT INTO whistleblower_tips (
			tip_reference, category, district_id,
			gwl_account_number, description,
			photo_urls, location_lat, location_lng,
			contact_phone, contact_method,
			reward_eligible,
			submission_ip_hash, user_agent_hash,
			status
		) VALUES ($1,$2::tip_category,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'NEW')
		RETURNING id
	`,
		tipRef, req.Category, districtID,
		req.GWLAccountNumber, req.Description,
		req.PhotoURLs, req.LocationLat, req.LocationLng,
		req.ContactPhone, req.ContactMethod,
		rewardEligible,
		ipHash, uaHash,
	).Scan(&tipID)

	if err != nil {
		h.logger.Error("Failed to save tip", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{"error": "failed to submit tip"})
	}

	h.logger.Info("Whistleblower tip received",
		zap.String("tip_ref", tipRef),
		zap.String("category", req.Category),
		zap.Bool("reward_eligible", rewardEligible),
	)

	// Build response in requested language
	responseMsg := h.getTipConfirmationMessage(req.Language, tipRef)

	return c.Status(201).JSON(fiber.Map{
		"tip_reference":   tipRef,
		"message":         responseMsg,
		"reward_eligible": rewardEligible,
		"note":            "Keep your reference number. You can check status at /api/v1/tips/" + tipRef,
	})
}

// ── GET /api/v1/tips/:ref (PUBLIC) ───────────────────────────────────────────
// Check tip status by reference number (no sensitive data returned)

func (h *WhistleblowerHandler) GetTipStatus(c *fiber.Ctx) error {
	tipRef := c.Params("ref")
	if tipRef == "" {
		return c.Status(400).JSON(fiber.Map{"error": "tip reference required"})
	}

	var status, category string
	var createdAt time.Time
	var rewardEligible bool
	var rewardAmountGHS *float64

	err := h.db.QueryRow(c.Context(), `
		SELECT status::text, category::text, created_at, reward_eligible, reward_amount_ghs
		FROM whistleblower_tips
		WHERE tip_reference = $1
	`, tipRef).Scan(&status, &category, &createdAt, &rewardEligible, &rewardAmountGHS)

	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "tip not found"})
	}

	return c.JSON(fiber.Map{
		"tip_reference":     tipRef,
		"status":            status,
		"category":          category,
		"submitted_at":      createdAt.Format("2006-01-02"),
		"reward_eligible":   rewardEligible,
		"reward_amount_ghs": rewardAmountGHS,
	})
}

// ── GET /api/v1/admin/tips (ADMIN) ───────────────────────────────────────────
// List all tips with investigation status

func (h *WhistleblowerHandler) ListTips(c *fiber.Ctx) error {
	statusFilter := c.Query("status")
	categoryFilter := c.Query("category")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	offset := (page - 1) * limit

	query := `
		SELECT
			wt.id, wt.tip_reference, wt.category::text, wt.status::text,
			wt.district_id, d.district_name,
			wt.gwl_account_number, wt.description,
			wt.reward_eligible, wt.reward_amount_ghs,
			wt.linked_audit_event_id,
			wt.created_at, wt.updated_at
		FROM whistleblower_tips wt
		LEFT JOIN districts d ON d.id = wt.district_id
		WHERE 1=1
	`
	// Build filter conditions for both the main query and the count query.
	// A dedicated countArgs slice is used instead of args[:argIdx-1] to avoid
	// off-by-one bugs when LIMIT/OFFSET are appended to args later (NEW-1).
	args := []interface{}{}
	countArgs := []interface{}{}
	argIdx := 1

	if statusFilter != "" {
		query += fmt.Sprintf(" AND wt.status = $%d::tip_status", argIdx)
		args = append(args, statusFilter)
		countArgs = append(countArgs, statusFilter)
		argIdx++
	}
	if categoryFilter != "" {
		query += fmt.Sprintf(" AND wt.category = $%d::tip_category", argIdx)
		args = append(args, categoryFilter)
		countArgs = append(countArgs, categoryFilter)
		argIdx++
	}

	// COUNT query reuses the same parameterised conditions via countArgs.
	countQuery := `SELECT COUNT(*) FROM whistleblower_tips wt LEFT JOIN districts d ON d.id = wt.district_id WHERE 1=1`
	if statusFilter != "" {
		countQuery += fmt.Sprintf(" AND wt.status = $1::tip_status")
	}
	if categoryFilter != "" {
		countArgIdx := 1
		if statusFilter != "" { countArgIdx = 2 }
		countQuery += fmt.Sprintf(" AND wt.category = $%d::tip_category", countArgIdx)
	}
	var total int
	_ = h.db.QueryRow(c.Context(), countQuery, countArgs...).Scan(&total)

	query += fmt.Sprintf(" ORDER BY wt.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := h.db.Query(c.Context(), query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch tips"})
	}
	defer rows.Close()

	type tipRow struct {
		ID                  string   `json:"id"`
		TipReference        string   `json:"tip_reference"`
		Category            string   `json:"category"`
		Status              string   `json:"status"`
		DistrictID          *string  `json:"district_id"`
		DistrictName        *string  `json:"district_name"`
		GWLAccountNumber    *string  `json:"gwl_account_number"`
		Description         string   `json:"description"`
		RewardEligible      bool     `json:"reward_eligible"`
		RewardAmountGHS     *float64 `json:"reward_amount_ghs"`
		LinkedAuditEventID  *string  `json:"linked_audit_event_id"`
		CreatedAt           string   `json:"created_at"`
		UpdatedAt           string   `json:"updated_at"`
	}

	var tips []tipRow
	for rows.Next() {
		var t tipRow
		var id uuid.UUID
		var districtID *uuid.UUID
		var linkedAuditID *uuid.UUID
		var createdAt, updatedAt time.Time

		if err := rows.Scan(
			&id, &t.TipReference, &t.Category, &t.Status,
			&districtID, &t.DistrictName,
			&t.GWLAccountNumber, &t.Description,
			&t.RewardEligible, &t.RewardAmountGHS,
			&linkedAuditID,
			&createdAt, &updatedAt,
		); err != nil {
			continue
		}
		t.ID = id.String()
		if districtID != nil {
			s := districtID.String()
			t.DistrictID = &s
		}
		if linkedAuditID != nil {
			s := linkedAuditID.String()
			t.LinkedAuditEventID = &s
		}
		t.CreatedAt = createdAt.Format(time.RFC3339)
		t.UpdatedAt = updatedAt.Format(time.RFC3339)
		tips = append(tips, t)
	}

	pages := (total + limit - 1) / limit
	if pages < 1 {
		pages = 1
	}
	return c.JSON(fiber.Map{
		"tips":  tips,
		"total": total,
		"page":  page,
		"limit": limit,
		"pages": pages,
	})
}

// ── PATCH /api/v1/admin/tips/:id (ADMIN) ─────────────────────────────────────
// Update tip investigation status

func (h *WhistleblowerHandler) UpdateTip(c *fiber.Ctx) error {
	tipID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid tip ID"})
	}

	type updateReq struct {
		Status              string   `json:"status"`
		InvestigationNotes  string   `json:"investigation_notes"`
		LinkedAuditEventID  string   `json:"linked_audit_event_id"`
		RewardAmountGHS     *float64 `json:"reward_amount_ghs"`
	}

	var req updateReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	userIDStr, _ := c.Locals("user_id").(string)
	userID, _ := uuid.Parse(userIDStr)

	var linkedAuditID *uuid.UUID
	if req.LinkedAuditEventID != "" {
		id, err := uuid.Parse(req.LinkedAuditEventID)
		if err == nil {
			linkedAuditID = &id
		}
	}

	_, err = h.db.Exec(c.Context(), `
		UPDATE whistleblower_tips SET
			status                = COALESCE(NULLIF($1,''), status::text)::tip_status,
			investigation_notes   = COALESCE(NULLIF($2,''), investigation_notes),
			linked_audit_event_id = COALESCE($3, linked_audit_event_id),
			reward_amount_ghs     = COALESCE($4, reward_amount_ghs),
			assigned_to           = $5,
			updated_at            = NOW()
		WHERE id = $6
	`, req.Status, req.InvestigationNotes, linkedAuditID, req.RewardAmountGHS, userID, tipID)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to update tip: " + err.Error()})
	}

	h.logger.Info("Tip updated",
		zap.String("tip_id", tipID.String()),
		zap.String("status", req.Status),
	)

	return c.JSON(fiber.Map{"message": "Tip updated", "id": tipID.String()})
}

// ── HELPERS ───────────────────────────────────────────────────────────────────

func (h *WhistleblowerHandler) getTipConfirmationMessage(lang, tipRef string) string {
	switch lang {
	case "tw": // Twi (Akan)
		return fmt.Sprintf("Wo nsɛm no ato mu. Ref: %s. Yɛbɛhwɛ na yɛbɛfrɛ wo sɛ ɛhia. Meda wo ase.", tipRef)
	case "ga": // Ga
		return fmt.Sprintf("Woyɛ akɛ ni shwane. Ref: %s. Mii bɛ lɛ ni. Oyiwaladon.", tipRef)
	default: // English
		return fmt.Sprintf("Your tip has been received. Reference: %s. We will investigate. Thank you for helping fight water fraud.", tipRef)
	}
}
