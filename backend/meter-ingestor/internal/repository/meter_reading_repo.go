package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// MeterReadingRecord is the DB-ready struct for a meter reading
type MeterReadingRecord struct {
	ID               uuid.UUID
	AccountID        uuid.UUID
	GWLAccountNumber string
	DeviceID         string
	ReadingM3        float64
	FlowRateM3H      float64
	PressureBar      float64
	ReadMethod       string
	ReaderID         string
	ReadingTimestamp time.Time
	BatteryVoltage   float64
	TamperDetected   bool
	DistrictCode     string
	CreatedAt        time.Time
	// Evidence fields — populated for FIELD_APP submissions, zero/empty for AMR/IoT
	GpsLat      float64
	GpsLng      float64
	GpsAccuracyM float64
	PhotoHash   string
}

// MeterReadingRepository handles persistence of meter readings
type MeterReadingRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewMeterReadingRepository(db *pgxpool.Pool, logger *zap.Logger) *MeterReadingRepository {
	return &MeterReadingRepository{db: db, logger: logger}
}

// Upsert inserts or updates a meter reading.
// ON CONFLICT on (account_id, reading_date) updates the reading values.
func (r *MeterReadingRepository) Upsert(ctx context.Context, rec *MeterReadingRecord) (uuid.UUID, error) {
	rec.ID = uuid.New()
	rec.CreatedAt = time.Now()

	// Resolve account_id from gwl_account_number
	var accountID uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT id FROM water_accounts WHERE gwl_account_number = $1`,
		rec.GWLAccountNumber,
	).Scan(&accountID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("account not found for GWL number %s: %w", rec.GWLAccountNumber, err)
	}
	rec.AccountID = accountID

	_, err = r.db.Exec(ctx, `
		INSERT INTO meter_readings (
			id, account_id, reading_date, reading_m3,
			flow_rate_m3h, pressure_bar,
			read_method, reader_id,
			battery_voltage, tamper_detected,
			created_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6,
			$7, $8,
			$9, $10,
			$11
		)
		ON CONFLICT (account_id, reading_date) DO UPDATE SET
			reading_m3      = EXCLUDED.reading_m3,
			flow_rate_m3h   = EXCLUDED.flow_rate_m3h,
			pressure_bar    = EXCLUDED.pressure_bar,
			read_method     = EXCLUDED.read_method,
			reader_id       = EXCLUDED.reader_id,
			battery_voltage = EXCLUDED.battery_voltage,
			tamper_detected = EXCLUDED.tamper_detected
	`,
		rec.ID, rec.AccountID, rec.ReadingTimestamp.Truncate(24*time.Hour), rec.ReadingM3,
		rec.FlowRateM3H, rec.PressureBar,
		rec.ReadMethod, rec.ReaderID,
		rec.BatteryVoltage, rec.TamperDetected,
		rec.CreatedAt,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert meter reading: %w", err)
	}

	r.logger.Debug("Meter reading upserted",
		zap.String("account", rec.GWLAccountNumber),
		zap.Float64("reading_m3", rec.ReadingM3),
		zap.Time("timestamp", rec.ReadingTimestamp),
	)

	return rec.ID, nil
}

// GetLatest returns the most recent reading for an account
func (r *MeterReadingRepository) GetLatest(ctx context.Context, gwlAccountNumber string) (*MeterReadingRecord, error) {
	rec := &MeterReadingRecord{}
	err := r.db.QueryRow(ctx, `
		SELECT mr.id, mr.account_id, wa.gwl_account_number,
		       mr.reading_date, mr.reading_m3,
		       COALESCE(mr.flow_rate_m3h, 0), COALESCE(mr.pressure_bar, 0),
		       mr.read_method, COALESCE(mr.reader_id, ''),
		       COALESCE(mr.battery_voltage, 0), COALESCE(mr.tamper_detected, false),
		       mr.created_at
		FROM meter_readings mr
		JOIN water_accounts wa ON wa.id = mr.account_id
		WHERE wa.gwl_account_number = $1
		ORDER BY mr.reading_date DESC
		LIMIT 1
	`, gwlAccountNumber).Scan(
		&rec.ID, &rec.AccountID, &rec.GWLAccountNumber,
		&rec.ReadingTimestamp, &rec.ReadingM3,
		&rec.FlowRateM3H, &rec.PressureBar,
		&rec.ReadMethod, &rec.ReaderID,
		&rec.BatteryVoltage, &rec.TamperDetected,
		&rec.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get latest reading: %w", err)
	}
	return rec, nil
}

// GetHistory returns readings for an account within a time range
func (r *MeterReadingRepository) GetHistory(
	ctx context.Context,
	gwlAccountNumber string,
	from, to time.Time,
	limit int,
) ([]*MeterReadingRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	rows, err := r.db.Query(ctx, `
		SELECT mr.id, mr.account_id, wa.gwl_account_number,
		       mr.reading_date, mr.reading_m3,
		       COALESCE(mr.flow_rate_m3h, 0), COALESCE(mr.pressure_bar, 0),
		       mr.read_method, COALESCE(mr.reader_id, ''),
		       COALESCE(mr.battery_voltage, 0), COALESCE(mr.tamper_detected, false),
		       mr.created_at
		FROM meter_readings mr
		JOIN water_accounts wa ON wa.id = mr.account_id
		WHERE wa.gwl_account_number = $1
		  AND mr.reading_date BETWEEN $2 AND $3
		ORDER BY mr.reading_date DESC
		LIMIT $4
	`, gwlAccountNumber, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("get reading history: %w", err)
	}
	defer rows.Close()

	var records []*MeterReadingRecord
	for rows.Next() {
		rec := &MeterReadingRecord{}
		if err := rows.Scan(
			&rec.ID, &rec.AccountID, &rec.GWLAccountNumber,
			&rec.ReadingTimestamp, &rec.ReadingM3,
			&rec.FlowRateM3H, &rec.PressureBar,
			&rec.ReadMethod, &rec.ReaderID,
			&rec.BatteryVoltage, &rec.TamperDetected,
			&rec.CreatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}
