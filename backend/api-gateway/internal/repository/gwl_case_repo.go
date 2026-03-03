package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/rls"
	"go.uber.org/zap"
)

// ─── Domain types for GWL Case Management ────────────────────────────────────

// GWLCase is an enriched view of an anomaly_flag with all GWL workflow fields.
// This is what the GWL portal displays in its case queue.
type GWLCase struct {
	// Core anomaly flag fields
	ID               uuid.UUID  `json:"id"`
	AccountID        *uuid.UUID `json:"account_id"`
	DistrictID       uuid.UUID  `json:"district_id"`
	FlagType         string     `json:"anomaly_type"`  // DB column: anomaly_type
	Severity         string     `json:"alert_level"`  // DB column: alert_level
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	Evidence         []byte     `json:"evidence_data"`  // DB column: evidence_data (JSONB)
	EstimatedLossGHS float64    `json:"estimated_loss_ghs"`
	CreatedAt        time.Time  `json:"created_at"`

	// GWL workflow fields
	GWLStatus      string     `json:"gwl_status"`
	GWLAssignedTo  *uuid.UUID `json:"gwl_assigned_to_id"`
	GWLAssignedAt  *time.Time `json:"gwl_assigned_at"`
	GWLResolvedAt  *time.Time `json:"gwl_resolved_at"`
	GWLResolution  *string    `json:"gwl_resolution"`
	GWLNotes       *string    `json:"gwl_notes"`
	DaysOpen       int        `json:"days_open"`

	// Joined account fields
	AccountNumber   *string `json:"account_number"`
	AccountHolder   *string `json:"account_holder"`
	AccountCategory *string `json:"account_category"`
	MeterNumber     *string `json:"meter_number"`
	Address         *string `json:"address"`

	// Joined district fields
	DistrictName string `json:"district_name"`
	DistrictCode string `json:"district_code"`
	Region       string `json:"region"`

	// Joined field officer fields
	AssignedOfficerName  *string `json:"assigned_officer_name"`
	AssignedOfficerEmail *string `json:"assigned_officer_email"`

	// Latest field job status
	FieldJobStatus *string `json:"field_job_status"`
	FieldJobID     *uuid.UUID `json:"field_job_id"`
}

// GWLCaseFilter defines filter parameters for the case queue
type GWLCaseFilter struct {
	DistrictID   *uuid.UUID
	FlagType     string  // maps to anomaly_type column
	Severity     string
	GWLStatus    string
	AssignedToID *uuid.UUID
	DateFrom     *time.Time
	DateTo       *time.Time
	Search       string // searches account number, holder name
	Limit        int
	Offset       int
	SortBy       string // estimated_loss_ghs, created_at, days_open
	SortDir      string // ASC, DESC
}

// GWLCaseSummary is the KPI strip data for the dashboard
type GWLCaseSummary struct {
	TotalOpen            int     `json:"total_open"`
	CriticalOpen         int     `json:"critical_open"`
	PendingReview        int     `json:"pending_review"`
	FieldAssigned        int     `json:"field_assigned"`
	ResolvedThisMonth    int     `json:"resolved_this_month"`
	TotalEstimatedLoss   float64 `json:"total_estimated_loss_ghs"`
	UnderbillingTotal    float64 `json:"underbilling_total_ghs"`
	OverbillingTotal     float64 `json:"overbilling_total_ghs"`
	MisclassifiedCount   int     `json:"misclassified_count"`
}

// ReclassificationRequest represents a category change request
type ReclassificationRequest struct {
	ID                      uuid.UUID  `json:"id"`
	AnomalyFlagID           uuid.UUID  `json:"anomaly_flag_id"`
	AccountID               uuid.UUID  `json:"account_id"`
	DistrictID              uuid.UUID  `json:"district_id"`
	CurrentCategory         string     `json:"current_category"`
	RecommendedCategory     string     `json:"recommended_category"`
	Justification           string     `json:"justification"`
	SupportingEvidence      []byte     `json:"supporting_evidence"`
	MonthlyRevenueImpact    float64    `json:"monthly_revenue_impact_ghs"`
	AnnualRevenueImpact     float64    `json:"annual_revenue_impact_ghs"`
	Status                  string     `json:"status"`
	RequestedByID           *uuid.UUID `json:"requested_by_id"`
	RequestedByName         string     `json:"requested_by_name"`
	ApprovedByID            *uuid.UUID `json:"approved_by_id"`
	ApprovedByName          *string    `json:"approved_by_name"`
	ApprovedAt              *time.Time `json:"approved_at"`
	AppliedInGWLAt          *time.Time `json:"applied_in_gwl_at"`
	GWLReference            *string    `json:"gwl_reference"`
	CreatedAt               time.Time  `json:"created_at"`
	// Joined
	AccountNumber   *string `json:"account_number"`
	AccountHolder   *string `json:"account_holder"`
	DistrictName    string  `json:"district_name"`
}

// CreditRequest represents an overbilling credit request
type CreditRequest struct {
	ID                   uuid.UUID  `json:"id"`
	AnomalyFlagID        uuid.UUID  `json:"anomaly_flag_id"`
	AccountID            uuid.UUID  `json:"account_id"`
	DistrictID           uuid.UUID  `json:"district_id"`
	GWLBillID            *uuid.UUID `json:"gwl_bill_id"`
	BillingPeriodStart   time.Time  `json:"billing_period_start"`
	BillingPeriodEnd     time.Time  `json:"billing_period_end"`
	GWLAmountGHS         float64    `json:"gwl_amount_ghs"`
	ShadowAmountGHS      float64    `json:"shadow_amount_ghs"`
	OverchargeAmountGHS  float64    `json:"overcharge_amount_ghs"`
	CreditAmountGHS      float64    `json:"credit_amount_ghs"`
	Reason               string     `json:"reason"`
	Notes                *string    `json:"notes"`
	Status               string     `json:"status"`
	RequestedByName      string     `json:"requested_by_name"`
	ApprovedByName       *string    `json:"approved_by_name"`
	ApprovedAt           *time.Time `json:"approved_at"`
	AppliedInGWLAt       *time.Time `json:"applied_in_gwl_at"`
	GWLCreditReference   *string    `json:"gwl_credit_reference"`
	CreatedAt            time.Time  `json:"created_at"`
	// Joined
	AccountNumber  *string `json:"account_number"`
	AccountHolder  *string `json:"account_holder"`
	DistrictName   string  `json:"district_name"`
}

// CaseAction is an entry in the case audit trail
type CaseAction struct {
	ID             uuid.UUID `json:"id"`
	AnomalyFlagID  uuid.UUID `json:"anomaly_flag_id"`
	PerformedByName string   `json:"performed_by_name"`
	PerformedByRole string   `json:"performed_by_role"`
	ActionType     string    `json:"action_type"`
	ActionNotes    *string   `json:"action_notes"`
	ActionMetadata []byte    `json:"action_metadata"`
	CreatedAt      time.Time `json:"created_at"`
}

// ─── Repository ───────────────────────────────────────────────────────────────

type GWLCaseRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewGWLCaseRepository(db *pgxpool.Pool, logger *zap.Logger) *GWLCaseRepository {
	return &GWLCaseRepository{db: db, logger: logger}
}

// q returns the Querier to use for this request.
// If an RLS-activated transaction is stored in ctx (by rls.Middleware), it is
// returned so that all queries run within that transaction and RLS is enforced.
// Otherwise the connection pool is returned (RLS not enforced — ops alert).
func (r *GWLCaseRepository) q(ctx context.Context) Querier {
	if tx, ok := rls.TxFromContext(ctx); ok {
		return tx
	}
	return r.db
}


// ── GetCaseSummary returns KPI strip data for the dashboard ──────────────────
func (r *GWLCaseRepository) GetCaseSummary(ctx context.Context, districtID *uuid.UUID) (*GWLCaseSummary, error) {
	districtFilter := "TRUE"
	args := []interface{}{}
	if districtID != nil {
		districtFilter = "af.district_id = $1"
		args = append(args, *districtID)
	}

	query := fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE af.gwl_status NOT IN ('CORRECTED','CLOSED'))                    AS total_open,
			COUNT(*) FILTER (WHERE af.alert_level = 'CRITICAL' AND af.gwl_status NOT IN ('CORRECTED','CLOSED')) AS critical_open,
			COUNT(*) FILTER (WHERE af.gwl_status = 'PENDING_REVIEW')                               AS pending_review,
			COUNT(*) FILTER (WHERE af.gwl_status = 'FIELD_ASSIGNED')                               AS field_assigned,
			COUNT(*) FILTER (WHERE af.gwl_resolved_at >= date_trunc('month', NOW()))                AS resolved_this_month,
			COALESCE(SUM(af.estimated_loss_ghs) FILTER (WHERE af.gwl_status NOT IN ('CORRECTED','CLOSED')), 0) AS total_loss,
			COALESCE(SUM(af.estimated_loss_ghs) FILTER (WHERE af.anomaly_type IN ('BILLING_VARIANCE','PHANTOM_METER','NRW_SPIKE') AND af.gwl_status NOT IN ('CORRECTED','CLOSED')), 0) AS underbilling,
			COALESCE(SUM(af.estimated_loss_ghs) FILTER (WHERE af.anomaly_type = 'OVERBILLING' AND af.gwl_status NOT IN ('CORRECTED','CLOSED')), 0) AS overbilling,
			COUNT(*) FILTER (WHERE af.anomaly_type = 'CATEGORY_MISMATCH' AND af.gwl_status NOT IN ('CORRECTED','CLOSED')) AS misclassified
		FROM anomaly_flags af
		WHERE %s
	`, districtFilter)

	var s GWLCaseSummary
	err := r.q(ctx).QueryRow(ctx, query, args...).Scan(
		&s.TotalOpen, &s.CriticalOpen, &s.PendingReview, &s.FieldAssigned,
		&s.ResolvedThisMonth, &s.TotalEstimatedLoss, &s.UnderbillingTotal,
		&s.OverbillingTotal, &s.MisclassifiedCount,
	)
	return &s, err
}

// ── ListCases returns the paginated case queue ────────────────────────────────
func (r *GWLCaseRepository) ListCases(ctx context.Context, f GWLCaseFilter) ([]*GWLCase, int, error) {
	conditions := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if f.DistrictID != nil {
		conditions = append(conditions, fmt.Sprintf("af.district_id = $%d", argIdx))
		args = append(args, *f.DistrictID)
		argIdx++
	}
	if f.FlagType != "" {
		conditions = append(conditions, fmt.Sprintf("af.anomaly_type = $%d", argIdx))
		args = append(args, f.FlagType)
		argIdx++
	}
	if f.Severity != "" {
		conditions = append(conditions, fmt.Sprintf("af.alert_level = $%d", argIdx))
		args = append(args, f.Severity)
		argIdx++
	}
	if f.GWLStatus != "" {
		conditions = append(conditions, fmt.Sprintf("af.gwl_status = $%d", argIdx))
		args = append(args, f.GWLStatus)
		argIdx++
	}
	if f.AssignedToID != nil {
		conditions = append(conditions, fmt.Sprintf("af.gwl_assigned_to_id = $%d", argIdx))
		args = append(args, *f.AssignedToID)
		argIdx++
	}
	if f.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("af.created_at >= $%d", argIdx))
		args = append(args, *f.DateFrom)
		argIdx++
	}
	if f.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("af.created_at <= $%d", argIdx))
		args = append(args, *f.DateTo)
		argIdx++
	}
	if f.Search != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(wa.gwl_account_number ILIKE $%d OR wa.account_holder_name ILIKE $%d)",
			argIdx, argIdx,
		))
		args = append(args, "%"+f.Search+"%")
		argIdx++
	}

	where := ""
	for i, c := range conditions {
		if i == 0 {
			where = c
		} else {
			where += " AND " + c
		}
	}

	sortBy := "af.estimated_loss_ghs"
	if f.SortBy == "created_at" { sortBy = "af.created_at" }
	if f.SortBy == "days_open"  { sortBy = "days_open" }
	sortDir := "DESC"
	if f.SortDir == "ASC" { sortDir = "ASC" }

	limit := 50
	if f.Limit > 0 && f.Limit <= 200 { limit = f.Limit }
	offset := f.Offset

	// Count query
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM anomaly_flags af
		LEFT JOIN water_accounts wa ON wa.id = af.account_id
		WHERE %s
	`, where)
	var total int
	if err := r.q(ctx).QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count cases: %w", err)
	}

	// Data query
	args = append(args, limit, offset)
	dataQuery := fmt.Sprintf(`
		SELECT
			af.id, af.account_id, af.district_id,
			af.anomaly_type, af.alert_level, af.title, af.description,
			af.evidence_data, af.estimated_loss_ghs, af.created_at,
			af.gwl_status, af.gwl_assigned_to_id, af.gwl_assigned_at,
			af.gwl_resolved_at, af.gwl_resolution, af.gwl_notes,
			EXTRACT(DAY FROM NOW() - af.created_at)::int AS days_open,
			wa.gwl_account_number, wa.account_holder_name, wa.category,
			wa.meter_number, wa.address_line1,
			d.district_name, d.district_code, d.region,
			u.full_name AS officer_name, u.email AS officer_email,
			fj.status AS field_job_status, fj.id AS field_job_id
		FROM anomaly_flags af
		LEFT JOIN water_accounts wa ON wa.id = af.account_id
		JOIN districts d ON d.id = af.district_id
		LEFT JOIN users u ON u.id = af.gwl_assigned_to_id
		LEFT JOIN LATERAL (
			SELECT id, status FROM field_jobs
			WHERE account_id = af.account_id
			ORDER BY created_at DESC LIMIT 1
		) fj ON TRUE
		WHERE %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, where, sortBy, sortDir, argIdx, argIdx+1)

	rows, err := r.q(ctx).Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list cases: %w", err)
	}
	defer rows.Close()

	var cases []*GWLCase
	for rows.Next() {
		c := &GWLCase{}
		if err := rows.Scan(
			&c.ID, &c.AccountID, &c.DistrictID,
			&c.FlagType, &c.Severity, &c.Title, &c.Description,
			&c.Evidence, &c.EstimatedLossGHS, &c.CreatedAt,
			&c.GWLStatus, &c.GWLAssignedTo, &c.GWLAssignedAt,
			&c.GWLResolvedAt, &c.GWLResolution, &c.GWLNotes,
			&c.DaysOpen,
			&c.AccountNumber, &c.AccountHolder, &c.AccountCategory,
			&c.MeterNumber, &c.Address,
			&c.DistrictName, &c.DistrictCode, &c.Region,
			&c.AssignedOfficerName, &c.AssignedOfficerEmail,
			&c.FieldJobStatus, &c.FieldJobID,
		); err != nil {
			return nil, 0, fmt.Errorf("scan case: %w", err)
		}
		cases = append(cases, c)
	}
	return cases, total, rows.Err()
}

// ── GetCaseByID returns a single case with full detail ───────────────────────
func (r *GWLCaseRepository) GetCaseByID(ctx context.Context, id uuid.UUID) (*GWLCase, error) {
	c := &GWLCase{}
	err := r.q(ctx).QueryRow(ctx, `
		SELECT
			af.id, af.account_id, af.district_id,
			af.anomaly_type, af.alert_level, af.title, af.description,
			af.evidence_data, af.estimated_loss_ghs, af.created_at,
			af.gwl_status, af.gwl_assigned_to_id, af.gwl_assigned_at,
			af.gwl_resolved_at, af.gwl_resolution, af.gwl_notes,
			EXTRACT(DAY FROM NOW() - af.created_at)::int AS days_open,
			wa.gwl_account_number, wa.account_holder_name, wa.category,
			wa.meter_number, wa.address_line1,
			d.district_name, d.district_code, d.region,
			u.full_name AS officer_name, u.email AS officer_email,
			fj.status AS field_job_status, fj.id AS field_job_id
		FROM anomaly_flags af
		LEFT JOIN water_accounts wa ON wa.id = af.account_id
		JOIN districts d ON d.id = af.district_id
		LEFT JOIN users u ON u.id = af.gwl_assigned_to_id
		LEFT JOIN LATERAL (
			SELECT id, status FROM field_jobs
			WHERE account_id = af.account_id
			ORDER BY created_at DESC LIMIT 1
		) fj ON TRUE
		WHERE af.id = $1
	`, id).Scan(
		&c.ID, &c.AccountID, &c.DistrictID,
		&c.FlagType, &c.Severity, &c.Title, &c.Description,
		&c.Evidence, &c.EstimatedLossGHS, &c.CreatedAt,
		&c.GWLStatus, &c.GWLAssignedTo, &c.GWLAssignedAt,
		&c.GWLResolvedAt, &c.GWLResolution, &c.GWLNotes,
		&c.DaysOpen,
		&c.AccountNumber, &c.AccountHolder, &c.AccountCategory,
		&c.MeterNumber, &c.Address,
		&c.DistrictName, &c.DistrictCode, &c.Region,
		&c.AssignedOfficerName, &c.AssignedOfficerEmail,
		&c.FieldJobStatus, &c.FieldJobID,
	)
	if err != nil {
		return nil, fmt.Errorf("get case by id: %w", err)
	}
	return c, nil
}

// ── UpdateCaseStatus updates the GWL workflow status of a case ───────────────
func (r *GWLCaseRepository) UpdateCaseStatus(ctx context.Context,
	flagID uuid.UUID, status string, assignedToID *uuid.UUID,
	resolution, notes *string, performedByName, performedByRole, actionType, actionNotes string,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	now := time.Now()
	var resolvedAt *time.Time
	if status == "CORRECTED" || status == "CLOSED" || status == "DISPUTED" {
		resolvedAt = &now
	}

	var assignedAt *time.Time
	if assignedToID != nil {
		assignedAt = &now
	}

	_, err = tx.Exec(ctx, `
		UPDATE anomaly_flags SET
			gwl_status         = $2,
			gwl_assigned_to_id = COALESCE($3, gwl_assigned_to_id),
			gwl_assigned_at    = COALESCE($4, gwl_assigned_at),
			gwl_resolved_at    = $5,
			gwl_resolution     = COALESCE($6, gwl_resolution),
			gwl_notes          = COALESCE($7, gwl_notes)
		WHERE id = $1
	`, flagID, status, assignedToID, assignedAt, resolvedAt, resolution, notes)
	if err != nil {
		return fmt.Errorf("update case status: %w", err)
	}

	// Record the action in the audit trail
	_, err = tx.Exec(ctx, `
		INSERT INTO gwl_case_actions (
			anomaly_flag_id, performed_by_name, performed_by_role,
			action_type, action_notes
		) VALUES ($1, $2, $3, $4, $5)
	`, flagID, performedByName, performedByRole, actionType, actionNotes)
	if err != nil {
		return fmt.Errorf("insert case action: %w", err)
	}

	return tx.Commit(ctx)
}

// ── AssignToFieldOfficer assigns a case to a field officer and creates a job ─
func (r *GWLCaseRepository) AssignToFieldOfficer(ctx context.Context,
	flagID, officerID, accountID uuid.UUID,
	priority, jobType, title, description string,
	dueDate time.Time,
	performedByName, performedByRole string,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	now := time.Now()

	// Update the anomaly flag
	_, err = tx.Exec(ctx, `
		UPDATE anomaly_flags SET
			gwl_status         = 'FIELD_ASSIGNED',
			gwl_assigned_to_id = $2,
			gwl_assigned_at    = $3
		WHERE id = $1
	`, flagID, officerID, now)
	if err != nil {
		return fmt.Errorf("update flag for field assignment: %w", err)
	}

	// Create a field job using only columns that exist in the field_jobs schema.
	// job_type, title, description, due_date are not columns in field_jobs;
	// we store them in the notes field and use a generated job_reference.
	jobRef := fmt.Sprintf("FJ-GWL-%s", flagID.String()[:8])
	priorityInt := 5 // default medium
	switch priority {
	case "CRITICAL": priorityInt = 1
	case "HIGH":     priorityInt = 2
	case "MEDIUM":   priorityInt = 5
	case "LOW":      priorityInt = 8
	}
	jobNotes := fmt.Sprintf("[%s] %s — %s", jobType, title, description)
	_, err = tx.Exec(ctx, `
		INSERT INTO field_jobs (
			job_reference, account_id, district_id, assigned_officer_id,
			status, is_blind_audit,
			target_gps_lat, target_gps_lng, gps_fence_radius_m,
			priority, notes
		) VALUES (
			$1, $2, $3, $4,
			'ASSIGNED'::field_job_status, TRUE,
			0, 0, 50,
			$5, $6
		) ON CONFLICT (job_reference) DO NOTHING
	`, jobRef, accountID, flagID, officerID, priorityInt, jobNotes)
	if err != nil {
		return fmt.Errorf("create field job: %w", err)
	}

	// Record the action
	_, err = tx.Exec(ctx, `
		INSERT INTO gwl_case_actions (
			anomaly_flag_id, performed_by_name, performed_by_role,
			action_type, action_notes,
			action_metadata
		) VALUES ($1, $2, $3, 'ASSIGNED', $4,
			jsonb_build_object('officer_id', $5::text, 'due_date', $6::text)
		)
	`, flagID, performedByName, performedByRole,
		fmt.Sprintf("Assigned to field officer for %s", jobType),
		officerID.String(), dueDate.Format("2006-01-02"),
	)
	if err != nil {
		return fmt.Errorf("insert assignment action: %w", err)
	}

	return tx.Commit(ctx)
}

// ── CreateReclassificationRequest creates a category change request ──────────
func (r *GWLCaseRepository) CreateReclassificationRequest(ctx context.Context, req *ReclassificationRequest) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO reclassification_requests (
			id, anomaly_flag_id, account_id, district_id,
			current_category, recommended_category, justification,
			supporting_evidence, monthly_revenue_impact_ghs, annual_revenue_impact_ghs,
			status, requested_by_id, requested_by_name
		) VALUES (
			gen_random_uuid(), $1, $2, $3,
			$4, $5, $6,
			$7, $8, $9,
			'PENDING', $10, $11
		)
	`,
		req.AnomalyFlagID, req.AccountID, req.DistrictID,
		req.CurrentCategory, req.RecommendedCategory, req.Justification,
		req.SupportingEvidence, req.MonthlyRevenueImpact, req.AnnualRevenueImpact,
		req.RequestedByID, req.RequestedByName,
	)
	if err != nil {
		return fmt.Errorf("create reclassification request: %w", err)
	}

	// Update case status
	_, err = tx.Exec(ctx, `
		UPDATE anomaly_flags SET gwl_status = 'APPROVED_FOR_CORRECTION'
		WHERE id = $1
	`, req.AnomalyFlagID)
	if err != nil {
		return fmt.Errorf("update flag status: %w", err)
	}

	// Record action
	_, err = tx.Exec(ctx, `
		INSERT INTO gwl_case_actions (
			anomaly_flag_id, performed_by_name, performed_by_role,
			action_type, action_notes
		) VALUES ($1, $2, 'GWL_SUPERVISOR', 'APPROVED_RECLASSIFICATION', $3)
	`, req.AnomalyFlagID, req.RequestedByName,
		fmt.Sprintf("Reclassification requested: %s → %s", req.CurrentCategory, req.RecommendedCategory),
	)
	if err != nil {
		return fmt.Errorf("insert reclassification action: %w", err)
	}

	return tx.Commit(ctx)
}

// ── ListReclassificationRequests returns reclassification requests ────────────
func (r *GWLCaseRepository) ListReclassificationRequests(ctx context.Context, status string, districtID *uuid.UUID) ([]*ReclassificationRequest, error) {
	conditions := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if status != "" {
		conditions = append(conditions, fmt.Sprintf("rr.status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}
	if districtID != nil {
		conditions = append(conditions, fmt.Sprintf("rr.district_id = $%d", argIdx))
		args = append(args, *districtID)
		argIdx++
	}

	where := conditions[0]
	for _, c := range conditions[1:] {
		where += " AND " + c
	}

	rows, err := r.q(ctx).Query(ctx, fmt.Sprintf(`
		SELECT
			rr.id, rr.anomaly_flag_id, rr.account_id, rr.district_id,
			rr.current_category, rr.recommended_category, rr.justification,
			rr.supporting_evidence, rr.monthly_revenue_impact_ghs, rr.annual_revenue_impact_ghs,
			rr.status, rr.requested_by_id, rr.requested_by_name,
			rr.approved_by_id, rr.approved_by_name, rr.approved_at,
			rr.applied_in_gwl_at, rr.gwl_reference, rr.created_at,
			wa.gwl_account_number, wa.account_holder_name,
			d.district_name
		FROM reclassification_requests rr
		JOIN water_accounts wa ON wa.id = rr.account_id
		JOIN districts d ON d.id = rr.district_id
		WHERE %s
		ORDER BY rr.monthly_revenue_impact_ghs DESC
	`, where), args...)
	if err != nil {
		return nil, fmt.Errorf("list reclassification requests: %w", err)
	}
	defer rows.Close()

	var results []*ReclassificationRequest
	for rows.Next() {
		req := &ReclassificationRequest{}
		if err := rows.Scan(
			&req.ID, &req.AnomalyFlagID, &req.AccountID, &req.DistrictID,
			&req.CurrentCategory, &req.RecommendedCategory, &req.Justification,
			&req.SupportingEvidence, &req.MonthlyRevenueImpact, &req.AnnualRevenueImpact,
			&req.Status, &req.RequestedByID, &req.RequestedByName,
			&req.ApprovedByID, &req.ApprovedByName, &req.ApprovedAt,
			&req.AppliedInGWLAt, &req.GWLReference, &req.CreatedAt,
			&req.AccountNumber, &req.AccountHolder,
			&req.DistrictName,
		); err != nil {
			return nil, fmt.Errorf("scan reclassification: %w", err)
		}
		results = append(results, req)
	}
	return results, rows.Err()
}

// ── CreateCreditRequest creates an overbilling credit request ────────────────
func (r *GWLCaseRepository) CreateCreditRequest(ctx context.Context, req *CreditRequest) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO credit_requests (
			id, anomaly_flag_id, account_id, district_id,
			gwl_bill_id, billing_period_start, billing_period_end,
			gwl_amount_ghs, shadow_amount_ghs, overcharge_amount_ghs, credit_amount_ghs,
			reason, notes, status, requested_by_name
		) VALUES (
			gen_random_uuid(), $1, $2, $3,
			$4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, 'PENDING', $13
		)
	`,
		req.AnomalyFlagID, req.AccountID, req.DistrictID,
		req.GWLBillID, req.BillingPeriodStart, req.BillingPeriodEnd,
		req.GWLAmountGHS, req.ShadowAmountGHS, req.OverchargeAmountGHS, req.CreditAmountGHS,
		req.Reason, req.Notes, req.RequestedByName,
	)
	if err != nil {
		return fmt.Errorf("create credit request: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE anomaly_flags SET gwl_status = 'APPROVED_FOR_CORRECTION' WHERE id = $1
	`, req.AnomalyFlagID)
	if err != nil {
		return fmt.Errorf("update flag for credit: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO gwl_case_actions (
			anomaly_flag_id, performed_by_name, performed_by_role,
			action_type, action_notes
		) VALUES ($1, $2, 'GWL_SUPERVISOR', 'ISSUED_CREDIT', $3)
	`, req.AnomalyFlagID, req.RequestedByName,
		fmt.Sprintf("Credit request raised: GHS %.2f overcharge", req.OverchargeAmountGHS),
	)
	if err != nil {
		return fmt.Errorf("insert credit action: %w", err)
	}

	return tx.Commit(ctx)
}

// ── ListCreditRequests returns credit requests ────────────────────────────────
func (r *GWLCaseRepository) ListCreditRequests(ctx context.Context, status string, districtID *uuid.UUID) ([]*CreditRequest, error) {
	conditions := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if status != "" {
		conditions = append(conditions, fmt.Sprintf("cr.status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}
	if districtID != nil {
		conditions = append(conditions, fmt.Sprintf("cr.district_id = $%d", argIdx))
		args = append(args, *districtID)
		argIdx++
	}

	where := conditions[0]
	for _, c := range conditions[1:] {
		where += " AND " + c
	}

	rows, err := r.q(ctx).Query(ctx, fmt.Sprintf(`
		SELECT
			cr.id, cr.anomaly_flag_id, cr.account_id, cr.district_id,
			cr.gwl_bill_id, cr.billing_period_start, cr.billing_period_end,
			cr.gwl_amount_ghs, cr.shadow_amount_ghs, cr.overcharge_amount_ghs, cr.credit_amount_ghs,
			cr.reason, cr.notes, cr.status,
			cr.requested_by_name, cr.approved_by_name, cr.approved_at,
			cr.applied_in_gwl_at, cr.gwl_credit_reference, cr.created_at,
			wa.gwl_account_number, wa.account_holder_name,
			d.district_name
		FROM credit_requests cr
		JOIN water_accounts wa ON wa.id = cr.account_id
		JOIN districts d ON d.id = cr.district_id
		WHERE %s
		ORDER BY cr.overcharge_amount_ghs DESC
	`, where), args...)
	if err != nil {
		return nil, fmt.Errorf("list credit requests: %w", err)
	}
	defer rows.Close()

	var results []*CreditRequest
	for rows.Next() {
		req := &CreditRequest{}
		if err := rows.Scan(
			&req.ID, &req.AnomalyFlagID, &req.AccountID, &req.DistrictID,
			&req.GWLBillID, &req.BillingPeriodStart, &req.BillingPeriodEnd,
			&req.GWLAmountGHS, &req.ShadowAmountGHS, &req.OverchargeAmountGHS, &req.CreditAmountGHS,
			&req.Reason, &req.Notes, &req.Status,
			&req.RequestedByName, &req.ApprovedByName, &req.ApprovedAt,
			&req.AppliedInGWLAt, &req.GWLCreditReference, &req.CreatedAt,
			&req.AccountNumber, &req.AccountHolder,
			&req.DistrictName,
		); err != nil {
			return nil, fmt.Errorf("scan credit request: %w", err)
		}
		results = append(results, req)
	}
	return results, rows.Err()
}

// ── GetCaseActions returns the audit trail for a case ────────────────────────
func (r *GWLCaseRepository) GetCaseActions(ctx context.Context, flagID uuid.UUID) ([]*CaseAction, error) {
	rows, err := r.q(ctx).Query(ctx, `
		SELECT id, anomaly_flag_id, performed_by_name, performed_by_role,
		       action_type, action_notes, action_metadata, created_at
		FROM gwl_case_actions
		WHERE anomaly_flag_id = $1
		ORDER BY created_at ASC
	`, flagID)
	if err != nil {
		return nil, fmt.Errorf("get case actions: %w", err)
	}
	defer rows.Close()

	var actions []*CaseAction
	for rows.Next() {
		a := &CaseAction{}
		if err := rows.Scan(
			&a.ID, &a.AnomalyFlagID, &a.PerformedByName, &a.PerformedByRole,
			&a.ActionType, &a.ActionNotes, &a.ActionMetadata, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan action: %w", err)
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}

// ── GetMonthlyReport returns or generates a monthly report ───────────────────
func (r *GWLCaseRepository) GetMonthlyReport(ctx context.Context, period time.Time, districtID *uuid.UUID) (map[string]interface{}, error) {
	periodStart := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)

	districtFilter := "TRUE"
	args := []interface{}{periodStart}
	if districtID != nil {
		districtFilter = "af.district_id = $2"
		args = append(args, *districtID)
	}

	var report struct {
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

	err := r.q(ctx).QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*)                                                                    AS total_flagged,
			COUNT(*) FILTER (WHERE af.alert_level = 'CRITICAL')                           AS critical,
			COUNT(*) FILTER (WHERE af.gwl_status IN ('CORRECTED','CLOSED'))            AS resolved,
			COUNT(*) FILTER (WHERE af.gwl_status NOT IN ('CORRECTED','CLOSED','DISPUTED')) AS pending,
			COUNT(*) FILTER (WHERE af.gwl_status = 'DISPUTED')                         AS disputed,
			COALESCE(SUM(af.estimated_loss_ghs) FILTER (WHERE af.anomaly_type != 'OVERBILLING'), 0) AS underbilling,
			COALESCE(SUM(af.estimated_loss_ghs) FILTER (WHERE af.anomaly_type = 'OVERBILLING'), 0)  AS overbilling,
			COALESCE(SUM(af.estimated_loss_ghs) FILTER (WHERE af.gwl_status = 'CORRECTED'), 0)   AS recovered,
			0::numeric AS credits_issued,
			0::int AS reclass_requested,
			0::int AS reclass_applied,
			0::int AS field_assigned,
			0::int AS field_completed
		FROM anomaly_flags af
		WHERE date_trunc('month', af.created_at) = $1
		  AND %s
	`, districtFilter), args...).Scan(
		&report.TotalFlagged, &report.CriticalCases,
		&report.Resolved, &report.Pending, &report.Disputed,
		&report.TotalUnderbillingGHS, &report.TotalOverbillingGHS,
		&report.RevenueRecoveredGHS, &report.CreditsIssuedGHS,
		&report.ReclassRequested, &report.ReclassApplied,
		&report.FieldJobsAssigned, &report.FieldJobsCompleted,
	)
	if err != nil {
		return nil, fmt.Errorf("get monthly report: %w", err)
	}

	return map[string]interface{}{
		"period":     periodStart.Format("January 2006"),
		"generated":  time.Now(),
		"statistics": report,
	}, nil
}
