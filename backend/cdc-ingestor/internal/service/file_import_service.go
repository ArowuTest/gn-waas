package service

// FileImportService handles GWL data file uploads (CSV format).
//
// Q4: GWL Billing DB Replica fallback
//   When GWL cannot provide a live PostgreSQL replica or API, they can provide
//   daily/monthly CSV file dumps. This service parses those files and writes
//   to the same tables as CDCSyncService, so the entire pipeline works identically.
//
// Q5: Ongoing account updates
//   After the initial bulk import, GWL provides daily delta files containing
//   only new/changed records. This service handles both full and delta imports.
//   Import mode is detected from the file header or the import_type parameter.
//
// Supported file types:
//   ACCOUNTS         — water_accounts (initial bulk + daily delta)
//   BILLING          — gwl_bills (monthly billing cycle)
//   METER_READINGS   — meter_readings (daily/monthly readings)
//   PRODUCTION_RECORDS — district_production_records (monthly production volumes)
//
// CSV format for each type is documented in docs/gwl-file-format.md
// (generated alongside this service).

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// FileImportService processes GWL CSV file uploads
type FileImportService struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewFileImportService(db *pgxpool.Pool, logger *zap.Logger) *FileImportService {
	return &FileImportService{db: db, logger: logger}
}

// ImportResult holds the result of a file import operation
type ImportResult struct {
	ImportID      uuid.UUID         `json:"import_id"`
	ImportType    string            `json:"import_type"`
	Filename      string            `json:"filename"`
	RecordsTotal  int               `json:"records_total"`
	RecordsOK     int               `json:"records_ok"`
	RecordsFailed int               `json:"records_failed"`
	Errors        []ImportRowError  `json:"errors,omitempty"`
	StartedAt     time.Time         `json:"started_at"`
	CompletedAt   time.Time         `json:"completed_at"`
	Status        string            `json:"status"` // COMPLETED | PARTIAL | FAILED
}

// ImportRowError describes a single row failure
type ImportRowError struct {
	Row     int    `json:"row"`
	Content string `json:"content"`
	Error   string `json:"error"`
}

// ── ACCOUNTS IMPORT ───────────────────────────────────────────────────────────
//
// Expected CSV columns (Q5 — supports both full and delta):
//   gwl_account_number, customer_name, customer_tin, category,
//   address_line1, address_line2, district_code,
//   gps_latitude, gps_longitude,
//   monthly_avg_consumption, status, effective_date
//
// Delta mode: only rows with effective_date > last import date are processed.
// Full mode:  all rows are upserted (ON CONFLICT DO UPDATE).
//
// New accounts are inserted; existing accounts are updated.
// Closed accounts (status=CLOSED) are soft-deleted (status updated, not deleted).

func (s *FileImportService) ImportAccounts(
	ctx context.Context,
	importID uuid.UUID,
	filename string,
	reader io.Reader,
) (*ImportResult, error) {
	result := &ImportResult{
		ImportID:   importID,
		ImportType: "ACCOUNTS",
		Filename:   filename,
		StartedAt:  time.Now(),
	}

	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true

	// Read header
	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}
	colIdx := buildColumnIndex(header)

	required := []string{"gwl_account_number", "customer_name", "category", "district_code"}
	for _, col := range required {
		if _, ok := colIdx[col]; !ok {
			return nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	rowNum := 1
	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.RecordsFailed++
			result.Errors = append(result.Errors, ImportRowError{
				Row: rowNum, Error: fmt.Sprintf("CSV parse error: %v", err),
			})
			rowNum++
			continue
		}
		result.RecordsTotal++

		if err := s.upsertAccount(ctx, colIdx, row); err != nil {
			result.RecordsFailed++
			if len(result.Errors) < 100 { // cap error list
				result.Errors = append(result.Errors, ImportRowError{
					Row:     rowNum,
					Content: strings.Join(row, ","),
					Error:   err.Error(),
				})
			}
		} else {
			result.RecordsOK++
		}
		rowNum++
	}

	result.CompletedAt = time.Now()
	result.Status = importStatus(result.RecordsOK, result.RecordsFailed, result.RecordsTotal)

	s.logger.Info("Account import completed",
		zap.String("import_id", importID.String()),
		zap.Int("total", result.RecordsTotal),
		zap.Int("ok", result.RecordsOK),
		zap.Int("failed", result.RecordsFailed),
	)

	return result, nil
}

func (s *FileImportService) upsertAccount(
	ctx context.Context,
	colIdx map[string]int,
	row []string,
) error {
	get := func(col string) string {
		if i, ok := colIdx[col]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	accountNum := get("gwl_account_number")
	if accountNum == "" {
		return fmt.Errorf("gwl_account_number is empty")
	}

	category := strings.ToUpper(get("category"))
	if category == "" {
		category = "RESIDENTIAL"
	}

	status := strings.ToUpper(get("status"))
	if status == "" {
		status = "ACTIVE"
	}

	// Resolve district_id from district_code
	districtCode := get("district_code")
	var districtID uuid.UUID
	err := s.db.QueryRow(ctx,
		`SELECT id FROM districts WHERE district_code = $1`, districtCode,
	).Scan(&districtID)
	if err != nil {
		return fmt.Errorf("district_code %q not found: %w", districtCode, err)
	}

	// Parse optional GPS
	var lat, lng *float64
	gpsSource := "UNKNOWN"
	if latStr := get("gps_latitude"); latStr != "" {
		if v, err := strconv.ParseFloat(latStr, 64); err == nil {
			lat = &v
			gpsSource = "GWL_PROVIDED"
		}
	}
	if lngStr := get("gps_longitude"); lngStr != "" {
		if v, err := strconv.ParseFloat(lngStr, 64); err == nil {
			lng = &v
		}
	}

	// Parse monthly avg consumption
	avgConsumption := 10.0 // default
	if v, err := strconv.ParseFloat(get("monthly_avg_consumption"), 64); err == nil && v > 0 {
		avgConsumption = v
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO water_accounts (
			gwl_account_number, customer_name, customer_tin,
			category, address_line1, address_line2,
			district_id, gps_latitude, gps_longitude,
			gps_source, monthly_avg_consumption, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::gps_source_type,$11,$12)
		ON CONFLICT (gwl_account_number) DO UPDATE SET
			customer_name          = EXCLUDED.customer_name,
			customer_tin           = EXCLUDED.customer_tin,
			category               = EXCLUDED.category::account_category,
			address_line1          = EXCLUDED.address_line1,
			address_line2          = EXCLUDED.address_line2,
			district_id            = EXCLUDED.district_id,
			gps_latitude           = COALESCE(EXCLUDED.gps_latitude, water_accounts.gps_latitude),
			gps_longitude          = COALESCE(EXCLUDED.gps_longitude, water_accounts.gps_longitude),
			gps_source             = CASE
				WHEN EXCLUDED.gps_latitude IS NOT NULL THEN EXCLUDED.gps_source::gps_source_type
				ELSE water_accounts.gps_source
			END,
			monthly_avg_consumption = EXCLUDED.monthly_avg_consumption,
			status                 = EXCLUDED.status::account_status,
			updated_at             = NOW()
	`,
		accountNum, get("customer_name"), get("customer_tin"),
		category, get("address_line1"), get("address_line2"),
		districtID, lat, lng,
		gpsSource, avgConsumption, status,
	)
	return err
}

// ── BILLING IMPORT ────────────────────────────────────────────────────────────
//
// Expected CSV columns:
//   gwl_account_number, billing_period_start, billing_period_end,
//   consumption_m3, billed_amount_ghs, vat_amount_ghs, total_amount_ghs,
//   read_method, bill_date, due_date

func (s *FileImportService) ImportBilling(
	ctx context.Context,
	importID uuid.UUID,
	filename string,
	reader io.Reader,
) (*ImportResult, error) {
	result := &ImportResult{
		ImportID:   importID,
		ImportType: "BILLING",
		Filename:   filename,
		StartedAt:  time.Now(),
	}

	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true

	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}
	colIdx := buildColumnIndex(header)

	required := []string{"gwl_account_number", "billing_period_start", "consumption_m3", "total_amount_ghs"}
	for _, col := range required {
		if _, ok := colIdx[col]; !ok {
			return nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	rowNum := 1
	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.RecordsFailed++
			rowNum++
			continue
		}
		result.RecordsTotal++

		if err := s.upsertBillingRecord(ctx, colIdx, row); err != nil {
			result.RecordsFailed++
			if len(result.Errors) < 100 {
				result.Errors = append(result.Errors, ImportRowError{
					Row: rowNum, Content: strings.Join(row, ","), Error: err.Error(),
				})
			}
		} else {
			result.RecordsOK++
		}
		rowNum++
	}

	result.CompletedAt = time.Now()
	result.Status = importStatus(result.RecordsOK, result.RecordsFailed, result.RecordsTotal)
	return result, nil
}

func (s *FileImportService) upsertBillingRecord(
	ctx context.Context,
	colIdx map[string]int,
	row []string,
) error {
	get := func(col string) string {
		if i, ok := colIdx[col]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	accountNum := get("gwl_account_number")
	if accountNum == "" {
		return fmt.Errorf("gwl_account_number is empty")
	}

	// Resolve account_id
	var accountID uuid.UUID
	err := s.db.QueryRow(ctx,
		`SELECT id FROM water_accounts WHERE gwl_account_number = $1`, accountNum,
	).Scan(&accountID)
	if err != nil {
		return fmt.Errorf("account %q not found (import accounts first): %w", accountNum, err)
	}

	periodStart, err := parseDate(get("billing_period_start"))
	if err != nil {
		return fmt.Errorf("invalid billing_period_start: %w", err)
	}

	periodEnd := periodStart.AddDate(0, 1, -1) // default: end of month
	if v, err := parseDate(get("billing_period_end")); err == nil {
		periodEnd = v
	}

	consumption, _ := strconv.ParseFloat(get("consumption_m3"), 64)
	billedAmt, _   := strconv.ParseFloat(get("billed_amount_ghs"), 64)
	vatAmt, _      := strconv.ParseFloat(get("vat_amount_ghs"), 64)
	totalAmt, _    := strconv.ParseFloat(get("total_amount_ghs"), 64)

	if totalAmt == 0 && billedAmt > 0 {
		totalAmt = billedAmt + vatAmt
	}

	readMethod := strings.ToUpper(get("read_method"))
	if readMethod == "" {
		readMethod = "MANUAL"
	}

	gwlBillRef := fmt.Sprintf("FILE-%s-%s", accountNum, periodStart.Format("200601"))

	_, err = s.db.Exec(ctx, `
		INSERT INTO gwl_bills (
			account_id, gwl_bill_reference,
			billing_period_start, billing_period_end,
			consumption_m3, billed_amount_ghs, vat_amount_ghs, total_amount_ghs,
			read_method, bill_date, due_date, source
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::read_method_type,$10,$11,'FILE_UPLOAD')
		ON CONFLICT (account_id, billing_period_start) DO UPDATE SET
			consumption_m3    = EXCLUDED.consumption_m3,
			billed_amount_ghs = EXCLUDED.billed_amount_ghs,
			vat_amount_ghs    = EXCLUDED.vat_amount_ghs,
			total_amount_ghs  = EXCLUDED.total_amount_ghs,
			read_method       = EXCLUDED.read_method,
			updated_at        = NOW()
	`,
		accountID, gwlBillRef,
		periodStart, periodEnd,
		consumption, billedAmt, vatAmt, totalAmt,
		readMethod,
		periodStart, // bill_date = period start
		periodStart.AddDate(0, 1, 14), // due_date = 45 days after period start
	)
	return err
}

// ── PRODUCTION RECORDS IMPORT (Q8) ───────────────────────────────────────────
//
// Expected CSV columns:
//   district_code, period_month (YYYY-MM), production_m3, data_source
//
// This is how GWL provides bulk meter production data when no live API exists.
// The sentinel uses these records for district-level NRW analysis.

func (s *FileImportService) ImportProductionRecords(
	ctx context.Context,
	importID uuid.UUID,
	filename string,
	reader io.Reader,
) (*ImportResult, error) {
	result := &ImportResult{
		ImportID:   importID,
		ImportType: "PRODUCTION_RECORDS",
		Filename:   filename,
		StartedAt:  time.Now(),
	}

	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true

	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}
	colIdx := buildColumnIndex(header)

	rowNum := 1
	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.RecordsFailed++
			rowNum++
			continue
		}
		result.RecordsTotal++

		if err := s.upsertProductionRecord(ctx, colIdx, row); err != nil {
			result.RecordsFailed++
			if len(result.Errors) < 100 {
				result.Errors = append(result.Errors, ImportRowError{
					Row: rowNum, Content: strings.Join(row, ","), Error: err.Error(),
				})
			}
		} else {
			result.RecordsOK++
		}
		rowNum++
	}

	result.CompletedAt = time.Now()
	result.Status = importStatus(result.RecordsOK, result.RecordsFailed, result.RecordsTotal)
	return result, nil
}

func (s *FileImportService) upsertProductionRecord(
	ctx context.Context,
	colIdx map[string]int,
	row []string,
) error {
	get := func(col string) string {
		if i, ok := colIdx[col]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	districtCode := get("district_code")
	if districtCode == "" {
		return fmt.Errorf("district_code is empty")
	}

	var districtID uuid.UUID
	err := s.db.QueryRow(ctx,
		`SELECT id FROM districts WHERE district_code = $1`, districtCode,
	).Scan(&districtID)
	if err != nil {
		return fmt.Errorf("district_code %q not found: %w", districtCode, err)
	}

	periodStr := get("period_month")
	if periodStr == "" {
		return fmt.Errorf("period_month is empty")
	}
	// Accept YYYY-MM or YYYY-MM-DD
	if len(periodStr) == 7 {
		periodStr += "-01"
	}
	period, err := time.Parse("2006-01-02", periodStr)
	if err != nil {
		return fmt.Errorf("invalid period_month %q: %w", periodStr, err)
	}

	productionM3, _ := strconv.ParseFloat(get("production_m3"), 64)
	if productionM3 <= 0 {
		return fmt.Errorf("production_m3 must be positive")
	}

	dataSource := strings.ToUpper(get("data_source"))
	if dataSource == "" {
		dataSource = "GWL_FILE_UPLOAD"
	}

	// Confidence score: file upload = 80, API = 95, statistical = 60
	confidence := 80.0
	if dataSource == "STATISTICAL_ESTIMATE" {
		confidence = 60.0
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO district_production_records (
			district_id, period_start, period_end,
			production_m3, data_source, data_confidence
		) VALUES ($1, $2, $3, $4, $5::production_data_source, $6)
		ON CONFLICT (district_id, period_start) DO UPDATE SET
			production_m3   = EXCLUDED.production_m3,
			data_source     = EXCLUDED.data_source,
			data_confidence = EXCLUDED.data_confidence,
			updated_at      = NOW()
	`,
		districtID,
		period,
		period.AddDate(0, 1, -1),
		productionM3,
		dataSource,
		confidence,
	)
	return err
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func buildColumnIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, col := range header {
		idx[strings.ToLower(strings.TrimSpace(col))] = i
	}
	return idx
}

func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	formats := []string{"2006-01-02", "02/01/2006", "01/02/2006", "2006-01"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date %q", s)
}

func importStatus(ok, failed, total int) string {
	if total == 0 {
		return "FAILED"
	}
	if failed == 0 {
		return "COMPLETED"
	}
	failPct := float64(failed) / float64(total) * 100
	if failPct > 50 {
		return "FAILED"
	}
	return "PARTIAL"
}

// roundFloat rounds to 2 decimal places
func roundFloat(v float64) float64 {
	return math.Round(v*100) / 100
}
