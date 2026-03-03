package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// CDCSyncService handles Change Data Capture from the GWL read-only replica.
//
// Architecture:
//   - Connects to GWL PostgreSQL read-only replica (configured via gwl_schema_map.yaml)
//   - Uses timestamp-based CDC: queries for rows WHERE updated_at > last_sync_time
//   - Maps GWL columns → GN-WAAS schema using SchemaMapper
//   - Upserts into GN-WAAS tables (ON CONFLICT DO UPDATE)
//   - Records every sync run in cdc_sync_log for audit trail
//   - Falls back to SKIPPED status if GWL credentials are not yet configured
type CDCSyncService struct {
	mapper   *SchemaMapper
	gnwaasDB *pgxpool.Pool // GN-WAAS target database
	logger   *zap.Logger
}

func NewCDCSyncService(mapper *SchemaMapper, gnwaasDB *pgxpool.Pool, logger *zap.Logger) *CDCSyncService {
	return &CDCSyncService{
		mapper:   mapper,
		gnwaasDB: gnwaasDB,
		logger:   logger,
	}
}

// SyncStatus holds the result of a CDC sync operation
type SyncStatus struct {
	SyncType      string    `json:"sync_type"`
	StartedAt     time.Time `json:"started_at"`
	CompletedAt   time.Time `json:"completed_at"`
	RecordsSynced int       `json:"records_synced"`
	RecordsFailed int       `json:"records_failed"`
	Status        string    `json:"status"`
	ErrorMessage  string    `json:"error_message,omitempty"`
}

// RunSync performs a full CDC sync for the specified table type.
//
// syncType values: "accounts", "billing", "meters"
//
// Flow:
//  1. Check GWL DB is configured
//  2. Connect to GWL read-only replica
//  3. Fetch last successful sync timestamp from cdc_sync_log
//  4. Query GWL for rows changed since last sync
//  5. Map each row using SchemaMapper
//  6. Upsert into GN-WAAS database
//  7. Write sync result to cdc_sync_log
func (s *CDCSyncService) RunSync(ctx context.Context, syncType string) (*SyncStatus, error) {
	status := &SyncStatus{
		SyncType:  syncType,
		StartedAt: time.Now(),
		Status:    "RUNNING",
	}

	s.logger.Info("CDC sync started", zap.String("type", syncType))

	// ── Step 1: Check GWL DB is configured ───────────────────────────────────
	if !s.mapper.IsGWLConfigured() {
		status.Status = "SKIPPED"
		status.ErrorMessage = "GWL_DB_HOST not configured. Set GWL_DB_HOST, GWL_DB_USER, GWL_DB_PASSWORD in environment."
		status.CompletedAt = time.Now()
		s.logger.Warn("CDC sync skipped — GWL database not configured",
			zap.String("hint", "Set GWL_DB_HOST env var to enable live sync"))
		s.writeSyncLog(ctx, status)
		return status, nil
	}

	// ── Step 2: Connect to GWL read-only replica ──────────────────────────────
	gwlDSN := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		s.mapper.schemaMap.GWLDatabase.Host,
		s.mapper.schemaMap.GWLDatabase.Port,
		s.mapper.schemaMap.GWLDatabase.Name,
		s.mapper.schemaMap.GWLDatabase.User,
		s.mapper.schemaMap.GWLDatabase.Password,
		s.mapper.schemaMap.GWLDatabase.SSLMode,
	)

	gwlDB, err := pgxpool.New(ctx, gwlDSN)
	if err != nil {
		status.Status = "FAILED"
		status.ErrorMessage = fmt.Sprintf("GWL DB connection failed: %v", err)
		status.CompletedAt = time.Now()
		s.logger.Error("GWL DB connection failed", zap.Error(err))
		s.writeSyncLog(ctx, status)
		return status, nil
	}
	defer gwlDB.Close()

	if err := gwlDB.Ping(ctx); err != nil {
		status.Status = "FAILED"
		status.ErrorMessage = fmt.Sprintf("GWL DB ping failed: %v", err)
		status.CompletedAt = time.Now()
		s.logger.Error("GWL DB ping failed", zap.Error(err))
		s.writeSyncLog(ctx, status)
		return status, nil
	}

	s.logger.Info("Connected to GWL read-only replica",
		zap.String("host", s.mapper.schemaMap.GWLDatabase.Host))

	// ── Step 3: Get last successful sync timestamp ────────────────────────────
	lastSync := s.getLastSyncTime(ctx, syncType)
	s.logger.Info("CDC sync window",
		zap.String("type", syncType),
		zap.Time("since", lastSync),
	)

	// ── Step 4 & 5: Sync based on type ───────────────────────────────────────
	var synced, failed int
	var syncErr error

	switch syncType {
	case "accounts":
		synced, failed, syncErr = s.syncAccounts(ctx, gwlDB, lastSync)
	case "billing":
		synced, failed, syncErr = s.syncBillingRecords(ctx, gwlDB, lastSync)
	case "meters":
		synced, failed, syncErr = s.syncMeterReadings(ctx, gwlDB, lastSync)
	default:
		status.Status = "FAILED"
		status.ErrorMessage = fmt.Sprintf("unknown sync type: %s", syncType)
		status.CompletedAt = time.Now()
		s.writeSyncLog(ctx, status)
		return status, nil
	}

	status.RecordsSynced = synced
	status.RecordsFailed = failed
	status.CompletedAt = time.Now()

	if syncErr != nil {
		status.Status = "FAILED"
		status.ErrorMessage = syncErr.Error()
		s.logger.Error("CDC sync failed",
			zap.String("type", syncType),
			zap.Error(syncErr),
			zap.Int("synced", synced),
			zap.Int("failed", failed),
		)
	} else {
		status.Status = "COMPLETED"
		s.logger.Info("CDC sync completed",
			zap.String("type", syncType),
			zap.Int("synced", synced),
			zap.Int("failed", failed),
			zap.Duration("duration", status.CompletedAt.Sub(status.StartedAt)),
		)
	}

	// ── Step 7: Write sync log ────────────────────────────────────────────────
	s.writeSyncLog(ctx, status)
	return status, nil
}

// syncAccounts syncs GWL customer accounts → GN-WAAS water_accounts
func (s *CDCSyncService) syncAccounts(ctx context.Context, gwlDB *pgxpool.Pool, since time.Time) (int, int, error) {
	tableMap, ok := s.mapper.schemaMap.Tables["accounts"]
	if !ok || !tableMap.Enabled {
		return 0, 0, nil
	}

	sourceColumns := extractSourceColumns(tableMap.Fields)
	query := fmt.Sprintf(
		"SELECT %s FROM %s.%s WHERE updated_at > $1 ORDER BY updated_at ASC LIMIT 5000",
		joinColumns(sourceColumns),
		s.mapper.schemaMap.GWLDatabase.Schema,
		tableMap.SourceTable,
	)

	rows, err := gwlDB.Query(ctx, query, since)
	if err != nil {
		return 0, 0, fmt.Errorf("GWL accounts query failed: %w", err)
	}
	defer rows.Close()

	synced, failed := 0, 0
	for rows.Next() {
		rawRow, err := scanRowToMap(rows, sourceColumns)
		if err != nil {
			failed++
			s.logger.Warn("Failed to scan GWL account row", zap.Error(err))
			continue
		}

		mapped, err := s.mapper.MapAccountRecord(rawRow)
		if err != nil || mapped == nil {
			failed++
			continue
		}

		// Resolve district_id from district_code
		var districtID *string
		if mapped.DistrictCode != "" {
			var dID string
			err := s.gnwaasDB.QueryRow(ctx,
				"SELECT id::text FROM districts WHERE district_code = $1",
				mapped.DistrictCode,
			).Scan(&dID)
			if err == nil {
				districtID = &dID
			}
		}

		// Map GWL status to GN-WAAS status
		status := "ACTIVE"
		if mapped.Status == "INACTIVE" || mapped.Status == "DISCONNECTED" || mapped.Status == "CLOSED" {
			status = "INACTIVE"
		}

		var tin *string
		if mapped.AccountHolderTIN != "" {
			tin = &mapped.AccountHolderTIN
		}

		// BE-M02 fix: add explicit enum casts for category and status.
		// pgx cannot reliably infer custom enum types from plain strings;
		// without the cast, the INSERT fails with "column is of type account_category
		// but expression is of type text" on some pgx driver versions.
		_, err = s.gnwaasDB.Exec(ctx, `
			INSERT INTO water_accounts (
				gwl_account_number, account_holder_name, account_holder_tin,
				category, status, district_id, meter_number,
				address_line1, gps_latitude, gps_longitude,
				is_active, updated_at
			) VALUES ($1,$2,$3,$4::account_category,$5::account_status,$6::uuid,$7,$8,$9,$10,$11,NOW())
			ON CONFLICT (gwl_account_number) DO UPDATE SET
				account_holder_name = EXCLUDED.account_holder_name,
				account_holder_tin  = EXCLUDED.account_holder_tin,
				category            = EXCLUDED.category,
				status              = EXCLUDED.status,
				meter_number        = EXCLUDED.meter_number,
				address_line1       = EXCLUDED.address_line1,
				gps_latitude        = EXCLUDED.gps_latitude,
				gps_longitude       = EXCLUDED.gps_longitude,
				is_active           = EXCLUDED.is_active,
				updated_at          = NOW()`,
			mapped.GWLAccountNumber, mapped.AccountHolderName, tin,
			mapped.Category, status, districtID, mapped.MeterNumber,
			mapped.AddressLine1, mapped.GPSLatitude, mapped.GPSLongitude,
			status == "ACTIVE",
		)
		if err != nil {
			failed++
			s.logger.Warn("Failed to upsert account",
				zap.String("gwl_number", mapped.GWLAccountNumber),
				zap.Error(err),
			)
			continue
		}
		synced++
	}

	return synced, failed, rows.Err()
}

// syncBillingRecords syncs GWL billing records → GN-WAAS gwl_bills table
func (s *CDCSyncService) syncBillingRecords(ctx context.Context, gwlDB *pgxpool.Pool, since time.Time) (int, int, error) {
	tableMap, ok := s.mapper.schemaMap.Tables["billing"]
	if !ok || !tableMap.Enabled {
		return 0, 0, nil
	}

	sourceColumns := extractSourceColumns(tableMap.Fields)
	query := fmt.Sprintf(
		"SELECT %s FROM %s.%s WHERE bill_date > $1 ORDER BY bill_date ASC LIMIT 10000",
		joinColumns(sourceColumns),
		s.mapper.schemaMap.GWLDatabase.Schema,
		tableMap.SourceTable,
	)

	rows, err := gwlDB.Query(ctx, query, since)
	if err != nil {
		return 0, 0, fmt.Errorf("GWL billing query failed: %w", err)
	}
	defer rows.Close()

	synced, failed := 0, 0
	for rows.Next() {
		rawRow, err := scanRowToMap(rows, sourceColumns)
		if err != nil {
			failed++
			continue
		}

		mapped, err := s.mapper.MapBillingRecord(rawRow)
		if err != nil || mapped == nil {
			failed++
			continue
		}

		// Resolve account_id from gwl_account_number
		var accountID string
		err = s.gnwaasDB.QueryRow(ctx,
			"SELECT id::text FROM water_accounts WHERE gwl_account_number = $1",
			mapped.GWLAccountNumber,
		).Scan(&accountID)
		if err != nil {
			failed++
			s.logger.Warn("Account not found for billing record",
				zap.String("gwl_account", mapped.GWLAccountNumber))
			continue
		}

		rawJSON, _ := json.Marshal(rawRow)

		_, err = s.gnwaasDB.Exec(ctx, `
			INSERT INTO gwl_bills (
				gwl_bill_id, account_id, billing_period_start, billing_period_end,
				consumption_m3, billed_amount_ghs, vat_amount_ghs, total_amount_ghs,
				bill_date, payment_status, raw_gwl_data
			) VALUES ($1,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (gwl_bill_id) DO UPDATE SET
				consumption_m3    = EXCLUDED.consumption_m3,
				billed_amount_ghs = EXCLUDED.billed_amount_ghs,
				vat_amount_ghs    = EXCLUDED.vat_amount_ghs,
				total_amount_ghs  = EXCLUDED.total_amount_ghs,
				payment_status    = EXCLUDED.payment_status`,
			mapped.GWLBillID, accountID,
			mapped.BillingPeriodStart, mapped.BillingPeriodEnd,
			mapped.ConsumptionM3, mapped.GWLAmountGHS, mapped.GWLVatGHS, mapped.GWLTotalGHS,
			mapped.GWLReadDate, mapped.PaymentStatus, string(rawJSON),
		)
		if err != nil {
			failed++
			s.logger.Warn("Failed to upsert billing record",
				zap.String("gwl_bill_id", mapped.GWLBillID),
				zap.Error(err),
			)
			continue
		}
		synced++
	}

	return synced, failed, rows.Err()
}

// syncMeterReadings syncs GWL meter readings → GN-WAAS meter_readings table
func (s *CDCSyncService) syncMeterReadings(ctx context.Context, gwlDB *pgxpool.Pool, since time.Time) (int, int, error) {
	tableMap, ok := s.mapper.schemaMap.Tables["meters"]
	if !ok || !tableMap.Enabled {
		return 0, 0, nil
	}

	sourceColumns := extractSourceColumns(tableMap.Fields)
	query := fmt.Sprintf(
		"SELECT %s FROM %s.%s WHERE reading_date > $1 ORDER BY reading_date ASC LIMIT 10000",
		joinColumns(sourceColumns),
		s.mapper.schemaMap.GWLDatabase.Schema,
		tableMap.SourceTable,
	)

	rows, err := gwlDB.Query(ctx, query, since)
	if err != nil {
		return 0, 0, fmt.Errorf("GWL meter readings query failed: %w", err)
	}
	defer rows.Close()

	synced, failed := 0, 0
	for rows.Next() {
		rawRow, err := scanRowToMap(rows, sourceColumns)
		if err != nil {
			failed++
			continue
		}

		// Resolve account_id from meter_number
		meterNumber := toString(rawRow["meter_number"])
		if meterNumber == "" {
			failed++
			continue
		}

		var accountID string
		err = s.gnwaasDB.QueryRow(ctx,
			"SELECT id::text FROM water_accounts WHERE meter_number = $1",
			meterNumber,
		).Scan(&accountID)
		if err != nil {
			failed++
			continue
		}

		readingDate := toTime(rawRow["reading_date"])
		readingValue := toFloat64(rawRow["current_reading"])
		readingType := toString(rawRow["reading_type"])
		if readingType == "" {
			readingType = "MANUAL"
		}

		rawJSON, _ := json.Marshal(rawRow)

		_, err = s.gnwaasDB.Exec(ctx, `
			INSERT INTO meter_readings (
				account_id, reading_date, reading_value_m3,
				reading_type, raw_gwl_data
			) VALUES ($1::uuid,$2,$3,$4,$5)
			ON CONFLICT (account_id, reading_date) DO UPDATE SET
				reading_value_m3 = EXCLUDED.reading_value_m3,
				reading_type     = EXCLUDED.reading_type`,
			accountID, readingDate, readingValue, readingType, string(rawJSON),
		)
		if err != nil {
			failed++
			continue
		}
		synced++
	}

	return synced, failed, rows.Err()
}

// getLastSyncTime returns the timestamp of the last successful sync for this type.
// Defaults to 30 days ago if no previous sync exists.
func (s *CDCSyncService) getLastSyncTime(ctx context.Context, syncType string) time.Time {
	var lastSync time.Time
	err := s.gnwaasDB.QueryRow(ctx, `
		SELECT COALESCE(MAX(completed_at), NOW() - INTERVAL '30 days')
		FROM cdc_sync_log
		WHERE sync_type = $1 AND status = 'COMPLETED'`, syncType,
	).Scan(&lastSync)
	if err != nil {
		return time.Now().AddDate(0, 0, -30)
	}
	return lastSync
}

// writeSyncLog records the sync result in cdc_sync_log for audit trail
func (s *CDCSyncService) writeSyncLog(ctx context.Context, status *SyncStatus) {
	_, err := s.gnwaasDB.Exec(ctx, `
		INSERT INTO cdc_sync_log (
			sync_type, started_at, completed_at,
			records_synced, records_failed, status, error_message
		) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		status.SyncType, status.StartedAt, status.CompletedAt,
		status.RecordsSynced, status.RecordsFailed, status.Status, status.ErrorMessage,
	)
	if err != nil {
		s.logger.Warn("Failed to write CDC sync log", zap.Error(err))
	}
}

// scanRowToMap scans a pgx row into a map[column]value
func scanRowToMap(rows pgx.Rows, columns []string) (map[string]interface{}, error) {
	values, err := rows.Values()
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{}, len(columns))
	for i, col := range columns {
		if i < len(values) {
			result[col] = values[i]
		}
	}
	return result, nil
}

// joinColumns joins column names with commas for SQL SELECT
func joinColumns(cols []string) string {
	result := ""
	for i, c := range cols {
		if i > 0 {
			result += ", "
		}
		result += c
	}
	return result
}

// extractSourceColumns extracts source column names from field mappings
func extractSourceColumns(fields []FieldMapping) []string {
	cols := make([]string, 0, len(fields))
	for _, f := range fields {
		cols = append(cols, f.SourceColumn)
	}
	return cols
}
