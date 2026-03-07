package handler

// CompoundHouseHandler manages compound house / shared meter accounts.
//
// GHANA CONTEXT — Why compound houses matter:
//   In Ghana, especially in older urban areas (Accra, Kumasi, Tamale),
//   a "compound house" is a shared residential structure where multiple
//   families live around a central courtyard and share ONE water meter.
//
//   This creates major billing problems:
//   1. One bill for 10-20 families → disputes about who pays what
//   2. GWL cannot identify individual consumers
//   3. When one family doesn't pay, the whole compound is cut off
//   4. Fraud: compound master collects from families but doesn't pay GWL
//
//   GN-WAAS solution:
//   - Register compound as a "master account" with sub-accounts per family
//   - Split bill proportionally (by occupants, rooms, or equal split)
//   - Track individual family payment via MoMo
//   - Compound master gets a dashboard to manage sub-accounts
//
// Compound Split Methods:
//   - EQUAL: divide equally among all members
//   - BY_OCCUPANTS: proportional to number of occupants per unit
//   - BY_ROOMS: proportional to number of rooms per unit
//   - CUSTOM: manually set percentages

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// CompoundHouseHandler manages compound house groups
type CompoundHouseHandler struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewCompoundHouseHandler(db *pgxpool.Pool, logger *zap.Logger) *CompoundHouseHandler {
	return &CompoundHouseHandler{db: db, logger: logger}
}

// ── POST /api/v1/admin/compounds ─────────────────────────────────────────────
// Create a compound house group

func (h *CompoundHouseHandler) CreateCompound(c *fiber.Ctx) error {
	type createReq struct {
		MasterAccountID string  `json:"master_account_id"` // existing water_account
		CompoundName    string  `json:"compound_name"`
		SplitMethod     string  `json:"split_method"` // EQUAL, BY_OCCUPANTS, BY_ROOMS, CUSTOM
		TotalOccupants  int     `json:"total_occupants"`
		TotalRooms      int     `json:"total_rooms"`
		Notes           string  `json:"notes"`
	}

	var req createReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	masterID, err := uuid.Parse(req.MasterAccountID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid master_account_id"})
	}

	if req.SplitMethod == "" {
		req.SplitMethod = "EQUAL"
	}

	// Verify master account exists
	var districtID uuid.UUID
	if err := h.db.QueryRow(c.Context(),
		`SELECT district_id FROM water_accounts WHERE id = $1`, masterID,
	).Scan(&districtID); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "master account not found"})
	}

	var groupID uuid.UUID
	err = h.db.QueryRow(c.Context(), `
		INSERT INTO compound_house_groups (
			master_account_id, district_id, compound_name,
			split_method, total_occupants, total_rooms, notes
		) VALUES ($1,$2,$3,$4::compound_split_method,$5,$6,$7)
		RETURNING id
	`,
		masterID, districtID, req.CompoundName,
		req.SplitMethod, req.TotalOccupants, req.TotalRooms, req.Notes,
	).Scan(&groupID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to create compound: " + err.Error()})
	}

	// Mark master account as compound master
	h.db.Exec(c.Context(), `
		UPDATE water_accounts SET
			is_compound_master = true,
			compound_group_id = $1,
			updated_at = NOW()
		WHERE id = $2
	`, groupID, masterID)

	h.logger.Info("Compound house created",
		zap.String("group_id", groupID.String()),
		zap.String("compound_name", req.CompoundName),
	)

	return c.Status(201).JSON(fiber.Map{
		"id":           groupID.String(),
		"compound_name": req.CompoundName,
		"split_method": req.SplitMethod,
	})
}

// ── POST /api/v1/admin/compounds/:id/members ─────────────────────────────────
// Add a member (sub-account) to a compound

func (h *CompoundHouseHandler) AddMember(c *fiber.Ctx) error {
	groupID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid compound ID"})
	}

	type addMemberReq struct {
		// Option A: link existing account
		AccountID string `json:"account_id"`
		// Option B: create new sub-account
		TenantName     string  `json:"tenant_name"`
		PhoneNumber    string  `json:"phone_number"`
		GhanaCardNum   string  `json:"ghana_card_number"`
		UnitNumber     string  `json:"unit_number"`
		Occupants      int     `json:"occupants"`
		Rooms          int     `json:"rooms"`
		SharePct       float64 `json:"share_pct"` // for CUSTOM split
	}

	var req addMemberReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Verify compound exists and get master account
	var masterAccountID, districtID uuid.UUID
	var splitMethod string
	if err := h.db.QueryRow(c.Context(), `
		SELECT master_account_id, district_id, split_method::text
		FROM compound_house_groups WHERE id = $1
	`, groupID).Scan(&masterAccountID, &districtID, &splitMethod); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "compound not found"})
	}

	var memberAccountID uuid.UUID

	if req.AccountID != "" {
		// Link existing account
		memberAccountID, err = uuid.Parse(req.AccountID)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid account_id"})
		}
	} else {
		// Create a new sub-account
		if req.TenantName == "" {
			return c.Status(400).JSON(fiber.Map{"error": "tenant_name required for new sub-account"})
		}

		// Get master account address for sub-account
		var masterAddr1, masterAddr2 string
		var masterLat, masterLng *float64
		h.db.QueryRow(c.Context(), `
			SELECT address_line1, address_line2, gps_latitude, gps_longitude
			FROM water_accounts WHERE id = $1
		`, masterAccountID).Scan(&masterAddr1, &masterAddr2, &masterLat, &masterLng)

		// Generate sub-account number: MASTER-SUB-NNN
		var masterAccNum string
		h.db.QueryRow(c.Context(),
			`SELECT gwl_account_number FROM water_accounts WHERE id = $1`, masterAccountID,
		).Scan(&masterAccNum)

		var memberCount int
		h.db.QueryRow(c.Context(),
			`SELECT COUNT(*) FROM compound_house_members WHERE compound_group_id = $1`, groupID,
		).Scan(&memberCount)

		subAccNum := fmt.Sprintf("%s-SUB-%03d", masterAccNum, memberCount+1)

		err = h.db.QueryRow(c.Context(), `
			INSERT INTO water_accounts (
				gwl_account_number, customer_name, district_id,
				address_line1, address_line2,
				gps_latitude, gps_longitude,
				phone_number, ghana_card_number,
				compound_group_id, is_compound_master,
				account_status
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,false,'ACTIVE')
			RETURNING id
		`,
			subAccNum, req.TenantName, districtID,
			masterAddr1, masterAddr2,
			masterLat, masterLng,
			req.PhoneNumber, req.GhanaCardNum,
			groupID,
		).Scan(&memberAccountID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to create sub-account: " + err.Error()})
		}
	}

	// Add to compound_house_members
	var memberID uuid.UUID
	err = h.db.QueryRow(c.Context(), `
		INSERT INTO compound_house_members (
			compound_group_id, account_id,
			unit_number, occupants, rooms, share_pct
		) VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id
	`,
		groupID, memberAccountID,
		req.UnitNumber, req.Occupants, req.Rooms, req.SharePct,
	).Scan(&memberID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to add member: " + err.Error()})
	}

	// Recalculate share percentages if not CUSTOM
	if splitMethod != "CUSTOM" {
		h.recalculateShares(c, groupID, splitMethod)
	}

	return c.Status(201).JSON(fiber.Map{
		"member_id":  memberID.String(),
		"account_id": memberAccountID.String(),
	})
}

// ── GET /api/v1/admin/compounds/:id ──────────────────────────────────────────
// Get compound details with members and bill split

func (h *CompoundHouseHandler) GetCompound(c *fiber.Ctx) error {
	groupID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid compound ID"})
	}

	var group struct {
		ID              string  `json:"id"`
		CompoundName    string  `json:"compound_name"`
		SplitMethod     string  `json:"split_method"`
		TotalOccupants  int     `json:"total_occupants"`
		TotalRooms      int     `json:"total_rooms"`
		MasterAccountID string  `json:"master_account_id"`
		MasterAccNum    string  `json:"master_account_number"`
		DistrictName    string  `json:"district_name"`
		CreatedAt       string  `json:"created_at"`
	}

	var gID, masterID uuid.UUID
	var createdAt time.Time
	if err := h.db.QueryRow(c.Context(), `
		SELECT
			chg.id, chg.compound_name, chg.split_method::text,
			chg.total_occupants, chg.total_rooms,
			chg.master_account_id, wa.gwl_account_number,
			d.district_name, chg.created_at
		FROM compound_house_groups chg
		JOIN water_accounts wa ON wa.id = chg.master_account_id
		JOIN districts d ON d.id = chg.district_id
		WHERE chg.id = $1
	`, groupID).Scan(
		&gID, &group.CompoundName, &group.SplitMethod,
		&group.TotalOccupants, &group.TotalRooms,
		&masterID, &group.MasterAccNum,
		&group.DistrictName, &createdAt,
	); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "compound not found"})
	}
	group.ID = gID.String()
	group.MasterAccountID = masterID.String()
	group.CreatedAt = createdAt.Format(time.RFC3339)

	// Get members
	memberRows, err := h.db.Query(c.Context(), `
		SELECT
			chm.id, chm.account_id, wa.gwl_account_number,
			wa.customer_name, wa.phone_number,
			chm.unit_number, chm.occupants, chm.rooms, chm.share_pct,
			chm.is_active
		FROM compound_house_members chm
		JOIN water_accounts wa ON wa.id = chm.account_id
		WHERE chm.compound_group_id = $1
		ORDER BY chm.unit_number
	`, groupID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch members"})
	}
	defer memberRows.Close()

	type memberRow struct {
		ID            string  `json:"id"`
		AccountID     string  `json:"account_id"`
		AccountNumber string  `json:"account_number"`
		CustomerName  string  `json:"customer_name"`
		PhoneNumber   string  `json:"phone_number"`
		UnitNumber    string  `json:"unit_number"`
		Occupants     int     `json:"occupants"`
		Rooms         int     `json:"rooms"`
		SharePct      float64 `json:"share_pct"`
		IsActive      bool    `json:"is_active"`
	}

	var members []memberRow
	for memberRows.Next() {
		var m memberRow
		var mID, aID uuid.UUID
		if err := memberRows.Scan(
			&mID, &aID, &m.AccountNumber,
			&m.CustomerName, &m.PhoneNumber,
			&m.UnitNumber, &m.Occupants, &m.Rooms, &m.SharePct,
			&m.IsActive,
		); err != nil {
			continue
		}
		m.ID = mID.String()
		m.AccountID = aID.String()
		members = append(members, m)
	}

	return c.JSON(fiber.Map{
		"compound": group,
		"members":  members,
		"member_count": len(members),
	})
}

// ── POST /api/v1/admin/compounds/:id/split-bill ───────────────────────────────
// Calculate and record the bill split for a given billing period

func (h *CompoundHouseHandler) SplitBill(c *fiber.Ctx) error {
	groupID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid compound ID"})
	}

	type splitReq struct {
		TotalBillGHS float64 `json:"total_bill_ghs"`
		BillingPeriod string `json:"billing_period"` // YYYY-MM
	}

	var req splitReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.TotalBillGHS <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "total_bill_ghs must be positive"})
	}

	// Get members with share percentages
	rows, err := h.db.Query(c.Context(), `
		SELECT chm.account_id, wa.customer_name, wa.phone_number, chm.share_pct
		FROM compound_house_members chm
		JOIN water_accounts wa ON wa.id = chm.account_id
		WHERE chm.compound_group_id = $1 AND chm.is_active = true
	`, groupID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch members"})
	}
	defer rows.Close()

	type memberSplit struct {
		AccountID    string  `json:"account_id"`
		CustomerName string  `json:"customer_name"`
		PhoneNumber  string  `json:"phone_number"`
		SharePct     float64 `json:"share_pct"`
		AmountGHS    float64 `json:"amount_ghs"`
	}

	var splits []memberSplit
	var totalSharePct float64

	for rows.Next() {
		var s memberSplit
		var aID uuid.UUID
		if err := rows.Scan(&aID, &s.CustomerName, &s.PhoneNumber, &s.SharePct); err != nil {
			continue
		}
		s.AccountID = aID.String()
		totalSharePct += s.SharePct
		splits = append(splits, s)
	}

	// Normalise if shares don't add to 100
	if totalSharePct > 0 && totalSharePct != 100 {
		for i := range splits {
			splits[i].SharePct = (splits[i].SharePct / totalSharePct) * 100
		}
	}

	// Calculate amounts
	var totalAllocated float64
	for i := range splits {
		splits[i].AmountGHS = req.TotalBillGHS * (splits[i].SharePct / 100)
		totalAllocated += splits[i].AmountGHS
	}

	// Rounding adjustment on last member
	if len(splits) > 0 {
		splits[len(splits)-1].AmountGHS += req.TotalBillGHS - totalAllocated
	}

	return c.JSON(fiber.Map{
		"compound_id":    groupID.String(),
		"billing_period": req.BillingPeriod,
		"total_bill_ghs": req.TotalBillGHS,
		"splits":         splits,
		"member_count":   len(splits),
	})
}

// recalculateShares recalculates share percentages for non-CUSTOM splits
func (h *CompoundHouseHandler) recalculateShares(c *fiber.Ctx, groupID uuid.UUID, splitMethod string) {
	rows, _ := h.db.Query(c.Context(), `
		SELECT id, occupants, rooms FROM compound_house_members
		WHERE compound_group_id = $1 AND is_active = true
	`, groupID)
	if rows == nil {
		return
	}
	defer rows.Close()

	type member struct {
		ID        uuid.UUID
		Occupants int
		Rooms     int
	}
	var members []member
	var totalOccupants, totalRooms int

	for rows.Next() {
		var m member
		rows.Scan(&m.ID, &m.Occupants, &m.Rooms)
		members = append(members, m)
		totalOccupants += m.Occupants
		totalRooms += m.Rooms
	}

	if len(members) == 0 {
		return
	}

	equalShare := 100.0 / float64(len(members))

	for _, m := range members {
		var sharePct float64
		switch splitMethod {
		case "EQUAL":
			sharePct = equalShare
		case "BY_OCCUPANTS":
			if totalOccupants > 0 {
				sharePct = float64(m.Occupants) / float64(totalOccupants) * 100
			} else {
				sharePct = equalShare
			}
		case "BY_ROOMS":
			if totalRooms > 0 {
				sharePct = float64(m.Rooms) / float64(totalRooms) * 100
			} else {
				sharePct = equalShare
			}
		default:
			sharePct = equalShare
		}

		h.db.Exec(c.Context(),
			`UPDATE compound_house_members SET share_pct = $1 WHERE id = $2`,
			sharePct, m.ID,
		)
	}
}
