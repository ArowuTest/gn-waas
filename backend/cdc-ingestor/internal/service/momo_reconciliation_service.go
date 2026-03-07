package service

// MoMoReconciliationService reconciles Mobile Money payments against GWL bills.
//
// GHANA CONTEXT — Why this matters:
//   Ghana's dominant payment method is Mobile Money (MoMo). Over 60% of GWL
//   customers pay their water bills via MTN MoMo, Vodafone Cash, or AirtelTigo Money.
//   GWL receives daily/weekly MoMo transaction exports from the providers.
//
//   Key fraud vectors this catches:
//   1. GHOST ACCOUNTS: MoMo payments for account numbers that don't exist in GWL
//      (someone is collecting payments for non-existent accounts)
//   2. OVERPAYMENT LAUNDERING: Payments significantly > bill amount
//      (possible money laundering through utility payments)
//   3. SPLIT PAYMENTS: Multiple small payments to avoid detection thresholds
//   4. UNMATCHED PAYMENTS: Payments received but GWL shows no bill
//      (bill may have been deleted/manipulated after payment)
//
// CSV Format (MTN MoMo export):
//   transaction_id, date, time, sender_phone, sender_name, amount, narration, status
//
// CSV Format (Vodafone Cash export):
//   ref_no, transaction_date, msisdn, customer_name, amount_ghs, description, status
//
// The service auto-detects the provider format from the CSV header.

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// MoMoReconciliationService processes MoMo payment exports
type MoMoReconciliationService struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewMoMoReconciliationService(db *pgxpool.Pool, logger *zap.Logger) *MoMoReconciliationService {
	return &MoMoReconciliationService{db: db, logger: logger}
}

// MoMoImportResult holds the result of a MoMo import
type MoMoImportResult struct {
	ImportID           uuid.UUID `json:"import_id"`
	Provider           string    `json:"provider"`
	Filename           string    `json:"filename"`
	RecordsTotal       int       `json:"records_total"`
	RecordsOK          int       `json:"records_ok"`
	RecordsFailed      int       `json:"records_failed"`
	// Reconciliation stats
	Matched            int       `json:"matched"`
	Unmatched          int       `json:"unmatched"`
	GhostAccounts      int       `json:"ghost_accounts"`
	Overpayments       int       `json:"overpayments"`
	Duplicates         int       `json:"duplicates"`
	FraudFlags         int       `json:"fraud_flags"`
	TotalAmountGHS     float64   `json:"total_amount_ghs"`
	MatchedAmountGHS   float64   `json:"matched_amount_ghs"`
	UnmatchedAmountGHS float64   `json:"unmatched_amount_ghs"`
	Errors             []ImportRowError `json:"errors,omitempty"`
	StartedAt          time.Time `json:"started_at"`
	CompletedAt        time.Time `json:"completed_at"`
	Status             string    `json:"status"`
}

// momoPaymentRecord is a normalised payment record from any provider
type momoPaymentRecord struct {
	TransactionID  string
	Provider       string
	SenderPhone    string
	SenderName     string
	AmountGHS      float64
	TransactionDate time.Time
	Narration      string
	RawRow         map[string]string
}

// ── MAIN IMPORT FUNCTION ──────────────────────────────────────────────────────

// ImportMoMoPayments processes a MoMo provider CSV export.
// Auto-detects provider format from CSV header.
func (s *MoMoReconciliationService) ImportMoMoPayments(
	ctx context.Context,
	importID uuid.UUID,
	filename string,
	reader io.Reader,
) (*MoMoImportResult, error) {
	result := &MoMoImportResult{
		ImportID:  importID,
		Filename:  filename,
		StartedAt: time.Now(),
	}

	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true

	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Auto-detect provider
	provider, parser := detectMoMoProvider(header)
	result.Provider = provider
	if parser == nil {
		return nil, fmt.Errorf("unrecognised MoMo CSV format. Headers: %v", header)
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

		payment, err := parser(colIdx, row)
		if err != nil {
			result.RecordsFailed++
			if len(result.Errors) < 100 {
				result.Errors = append(result.Errors, ImportRowError{
					Row: rowNum, Content: strings.Join(row, ","), Error: err.Error(),
				})
			}
			rowNum++
			continue
		}

		payment.Provider = provider

		// Process and reconcile
		reconcileResult, err := s.processPayment(ctx, importID, payment)
		if err != nil {
			result.RecordsFailed++
			if len(result.Errors) < 100 {
				result.Errors = append(result.Errors, ImportRowError{
					Row: rowNum, Content: strings.Join(row, ","), Error: err.Error(),
				})
			}
		} else {
			result.RecordsOK++
			result.TotalAmountGHS += payment.AmountGHS

			switch reconcileResult {
			case "MATCHED":
				result.Matched++
				result.MatchedAmountGHS += payment.AmountGHS
			case "UNMATCHED":
				result.Unmatched++
				result.UnmatchedAmountGHS += payment.AmountGHS
			case "GHOST_ACCOUNT":
				result.GhostAccounts++
				result.UnmatchedAmountGHS += payment.AmountGHS
				result.FraudFlags++
			case "OVERPAYMENT":
				result.Overpayments++
				result.MatchedAmountGHS += payment.AmountGHS
				result.FraudFlags++
			case "DUPLICATE":
				result.Duplicates++
			}
		}
		rowNum++
	}

	result.CompletedAt = time.Now()
	result.Status = importStatus(result.RecordsOK, result.RecordsFailed, result.RecordsTotal)

	s.logger.Info("MoMo import completed",
		zap.String("import_id", importID.String()),
		zap.String("provider", provider),
		zap.Int("total", result.RecordsTotal),
		zap.Int("matched", result.Matched),
		zap.Int("ghost_accounts", result.GhostAccounts),
		zap.Int("fraud_flags", result.FraudFlags),
	)

	return result, nil
}

// ── PAYMENT PROCESSING ────────────────────────────────────────────────────────

func (s *MoMoReconciliationService) processPayment(
	ctx context.Context,
	importID uuid.UUID,
	p *momoPaymentRecord,
) (string, error) {
	// 1. Check for duplicate
	var existingID uuid.UUID
	err := s.db.QueryRow(ctx,
		`SELECT id FROM mobile_money_payments WHERE provider = $1 AND transaction_id = $2`,
		p.Provider, p.TransactionID,
	).Scan(&existingID)
	if err == nil {
		// Already exists — mark as duplicate
		s.db.Exec(ctx,
			`UPDATE mobile_money_payments SET reconciliation_status = 'DUPLICATE' WHERE id = $1`,
			existingID,
		)
		return "DUPLICATE", nil
	}

	// 2. Extract GWL account number from narration
	gwlAccountNum := extractAccountNumber(p.Narration)

	// 3. Look up account
	var accountID *uuid.UUID
	var billID *uuid.UUID
	var billAmountGHS float64
	reconcileStatus := "UNMATCHED"
	isFraud := false
	fraudReason := ""
	var varianceGHS *float64

	if gwlAccountNum != "" {
		var accID uuid.UUID
		err := s.db.QueryRow(ctx,
			`SELECT id FROM water_accounts WHERE gwl_account_number = $1`,
			gwlAccountNum,
		).Scan(&accID)

		if err != nil {
			// Account number in narration but not in GWL — GHOST ACCOUNT
			reconcileStatus = "GHOST_ACCOUNT"
			isFraud = true
			fraudReason = fmt.Sprintf("Account %s not found in GWL database", gwlAccountNum)
		} else {
			accountID = &accID

			// 4. Find matching bill for this account around the payment date
			var bID uuid.UUID
			err = s.db.QueryRow(ctx, `
				SELECT id, total_amount_ghs
				FROM gwl_bills
				WHERE account_id = $1
				  AND billing_period_start <= $2
				  AND billing_period_end >= $2 - INTERVAL '45 days'
				ORDER BY ABS(EXTRACT(EPOCH FROM (billing_period_end - $2)))
				LIMIT 1
			`, accID, p.TransactionDate).Scan(&bID, &billAmountGHS)

			if err != nil {
				reconcileStatus = "UNMATCHED"
			} else {
				billID = &bID
				variance := p.AmountGHS - billAmountGHS
				varianceGHS = &variance

				if p.AmountGHS > billAmountGHS*1.20 {
					// Payment > 120% of bill — overpayment flag
					reconcileStatus = "OVERPAYMENT"
					isFraud = true
					fraudReason = fmt.Sprintf(
						"Payment GH₵%.2f is %.0f%% above bill GH₵%.2f",
						p.AmountGHS, (p.AmountGHS/billAmountGHS-1)*100, billAmountGHS,
					)
				} else if p.AmountGHS < billAmountGHS*0.50 {
					reconcileStatus = "UNDERPAYMENT"
				} else {
					reconcileStatus = "MATCHED"
				}
			}
		}
	}

	// 5. Insert payment record
	rawRowJSON := buildRawRowJSON(p)
	_, err = s.db.Exec(ctx, `
		INSERT INTO mobile_money_payments (
			transaction_id, provider, sender_phone, sender_name,
			amount_ghs, transaction_date, narration,
			gwl_account_number, account_id, gwl_bill_id,
			reconciliation_status, variance_ghs,
			is_fraud_flag, fraud_reason,
			import_batch_id, raw_row
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::momo_reconciliation_status,$12,$13,$14,$15,$16)
		ON CONFLICT (provider, transaction_id) DO NOTHING
	`,
		p.TransactionID, p.Provider, p.SenderPhone, p.SenderName,
		p.AmountGHS, p.TransactionDate, p.Narration,
		gwlAccountNum, accountID, billID,
		reconcileStatus, varianceGHS,
		isFraud, fraudReason,
		importID, rawRowJSON,
	)
	if err != nil {
		return "", fmt.Errorf("failed to insert payment: %w", err)
	}

	// 6. If fraud, create an anomaly flag
	if isFraud && accountID != nil {
		s.createFraudAnomaly(ctx, *accountID, p, reconcileStatus, fraudReason)
	}

	return reconcileStatus, nil
}

// createFraudAnomaly creates an anomaly flag for MoMo fraud
func (s *MoMoReconciliationService) createFraudAnomaly(
	ctx context.Context,
	accountID uuid.UUID,
	p *momoPaymentRecord,
	anomalyType string,
	reason string,
) {
	// Get district_id for the account
	var districtID uuid.UUID
	if err := s.db.QueryRow(ctx,
		`SELECT district_id FROM water_accounts WHERE id = $1`, accountID,
	).Scan(&districtID); err != nil {
		return
	}

	s.db.Exec(ctx, `
		INSERT INTO anomaly_flags (
			account_id, district_id, anomaly_type, severity,
			description, detected_at, status
		) VALUES ($1, $2, $3, 'HIGH', $4, NOW(), 'OPEN')
		ON CONFLICT DO NOTHING
	`,
		accountID, districtID,
		"MOMO_"+anomalyType,
		fmt.Sprintf("MoMo fraud signal: %s. Transaction: %s, Amount: GH₵%.2f",
			reason, p.TransactionID, p.AmountGHS),
	)
}

// ── PROVIDER FORMAT DETECTION ─────────────────────────────────────────────────

type momoParser func(colIdx map[string]int, row []string) (*momoPaymentRecord, error)

func detectMoMoProvider(header []string) (string, momoParser) {
	headerStr := strings.ToLower(strings.Join(header, ","))

	switch {
	case strings.Contains(headerStr, "transaction_id") && strings.Contains(headerStr, "sender_phone"):
		return "MTN_MOMO", parseMTNMoMo
	case strings.Contains(headerStr, "ref_no") && strings.Contains(headerStr, "msisdn"):
		return "VODAFONE_CASH", parseVodafoneCash
	case strings.Contains(headerStr, "reference") && strings.Contains(headerStr, "subscriber"):
		return "AIRTELTIGO_MONEY", parseAirtelTigoMoney
	case strings.Contains(headerStr, "transaction_ref") && strings.Contains(headerStr, "wallet"):
		return "G_MONEY", parseGMoney
	default:
		// Try generic format
		if strings.Contains(headerStr, "amount") && strings.Contains(headerStr, "phone") {
			return "UNKNOWN", parseGenericMoMo
		}
		return "UNKNOWN", nil
	}
}

// MTN MoMo CSV: transaction_id, date, time, sender_phone, sender_name, amount, narration, status
func parseMTNMoMo(colIdx map[string]int, row []string) (*momoPaymentRecord, error) {
	get := func(col string) string {
		if i, ok := colIdx[col]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	txID := get("transaction_id")
	if txID == "" {
		return nil, fmt.Errorf("transaction_id is empty")
	}

	amount, err := strconv.ParseFloat(strings.ReplaceAll(get("amount"), ",", ""), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %s", get("amount"))
	}

	// Parse date + time
	dateStr := get("date") + " " + get("time")
	txDate, err := parseDateTime(dateStr)
	if err != nil {
		txDate = time.Now()
	}

	return &momoPaymentRecord{
		TransactionID:   txID,
		SenderPhone:     normalisePhone(get("sender_phone")),
		SenderName:      get("sender_name"),
		AmountGHS:       amount,
		TransactionDate: txDate,
		Narration:       get("narration"),
		RawRow:          rowToMap(colIdx, row),
	}, nil
}

// Vodafone Cash CSV: ref_no, transaction_date, msisdn, customer_name, amount_ghs, description, status
func parseVodafoneCash(colIdx map[string]int, row []string) (*momoPaymentRecord, error) {
	get := func(col string) string {
		if i, ok := colIdx[col]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	txID := get("ref_no")
	if txID == "" {
		return nil, fmt.Errorf("ref_no is empty")
	}

	amount, err := strconv.ParseFloat(strings.ReplaceAll(get("amount_ghs"), ",", ""), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount_ghs: %s", get("amount_ghs"))
	}

	txDate, _ := parseDateTime(get("transaction_date"))

	return &momoPaymentRecord{
		TransactionID:   txID,
		SenderPhone:     normalisePhone(get("msisdn")),
		SenderName:      get("customer_name"),
		AmountGHS:       amount,
		TransactionDate: txDate,
		Narration:       get("description"),
		RawRow:          rowToMap(colIdx, row),
	}, nil
}

// AirtelTigo Money CSV: reference, transaction_date, subscriber, name, amount, narration
func parseAirtelTigoMoney(colIdx map[string]int, row []string) (*momoPaymentRecord, error) {
	get := func(col string) string {
		if i, ok := colIdx[col]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	txID := get("reference")
	if txID == "" {
		return nil, fmt.Errorf("reference is empty")
	}

	amount, err := strconv.ParseFloat(strings.ReplaceAll(get("amount"), ",", ""), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %s", get("amount"))
	}

	txDate, _ := parseDateTime(get("transaction_date"))

	return &momoPaymentRecord{
		TransactionID:   txID,
		SenderPhone:     normalisePhone(get("subscriber")),
		SenderName:      get("name"),
		AmountGHS:       amount,
		TransactionDate: txDate,
		Narration:       get("narration"),
		RawRow:          rowToMap(colIdx, row),
	}, nil
}

// G-Money CSV: transaction_ref, date, wallet, holder_name, amount, description
func parseGMoney(colIdx map[string]int, row []string) (*momoPaymentRecord, error) {
	get := func(col string) string {
		if i, ok := colIdx[col]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	txID := get("transaction_ref")
	if txID == "" {
		return nil, fmt.Errorf("transaction_ref is empty")
	}

	amount, err := strconv.ParseFloat(strings.ReplaceAll(get("amount"), ",", ""), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %s", get("amount"))
	}

	txDate, _ := parseDateTime(get("date"))

	return &momoPaymentRecord{
		TransactionID:   txID,
		SenderPhone:     normalisePhone(get("wallet")),
		SenderName:      get("holder_name"),
		AmountGHS:       amount,
		TransactionDate: txDate,
		Narration:       get("description"),
		RawRow:          rowToMap(colIdx, row),
	}, nil
}

// Generic MoMo parser for unknown formats
func parseGenericMoMo(colIdx map[string]int, row []string) (*momoPaymentRecord, error) {
	get := func(col string) string {
		if i, ok := colIdx[col]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}

	// Try common column name variations
	txID := firstNonEmpty(get("transaction_id"), get("ref_no"), get("reference"), get("id"))
	if txID == "" {
		return nil, fmt.Errorf("cannot find transaction ID column")
	}

	amountStr := firstNonEmpty(get("amount"), get("amount_ghs"), get("value"))
	amount, err := strconv.ParseFloat(strings.ReplaceAll(amountStr, ",", ""), 64)
	if err != nil {
		return nil, fmt.Errorf("cannot parse amount: %s", amountStr)
	}

	phone := firstNonEmpty(get("phone"), get("sender_phone"), get("msisdn"), get("subscriber"), get("wallet"))
	narration := firstNonEmpty(get("narration"), get("description"), get("memo"), get("reference_note"))
	dateStr := firstNonEmpty(get("date"), get("transaction_date"), get("created_at"))
	txDate, _ := parseDateTime(dateStr)

	return &momoPaymentRecord{
		TransactionID:   txID,
		SenderPhone:     normalisePhone(phone),
		AmountGHS:       amount,
		TransactionDate: txDate,
		Narration:       narration,
		RawRow:          rowToMap(colIdx, row),
	}, nil
}

// ── ACCOUNT NUMBER EXTRACTION ─────────────────────────────────────────────────

// GWL account numbers follow patterns like: GWL-ACC-12345, ACC-12345, 12345678
var accountPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)GWL[-\s]?ACC[-\s]?(\d{4,10})`),
	regexp.MustCompile(`(?i)ACC[-\s]?(\d{4,10})`),
	regexp.MustCompile(`(?i)ACCOUNT[-\s#:]*(\d{4,10})`),
	regexp.MustCompile(`(?i)A/C[-\s#:]*(\d{4,10})`),
	regexp.MustCompile(`\b(\d{8,10})\b`), // bare 8-10 digit number
}

func extractAccountNumber(narration string) string {
	for _, re := range accountPatterns {
		if m := re.FindStringSubmatch(narration); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

// ── HELPERS ───────────────────────────────────────────────────────────────────

// normalisePhone converts Ghana phone numbers to +233 format
func normalisePhone(phone string) string {
	phone = regexp.MustCompile(`[^\d+]`).ReplaceAllString(phone, "")
	if strings.HasPrefix(phone, "0") && len(phone) == 10 {
		return "+233" + phone[1:]
	}
	if strings.HasPrefix(phone, "233") && len(phone) == 12 {
		return "+" + phone
	}
	return phone
}

func parseDateTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"02/01/2006 15:04:05",
		"02/01/2006",
		"2006-01-02",
		"01/02/2006",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Now(), fmt.Errorf("cannot parse datetime: %s", s)
}

func rowToMap(colIdx map[string]int, row []string) map[string]string {
	m := make(map[string]string, len(colIdx))
	for col, idx := range colIdx {
		if idx < len(row) {
			m[col] = row[idx]
		}
	}
	return m
}

func buildRawRowJSON(p *momoPaymentRecord) string {
	parts := []string{}
	for k, v := range p.RawRow {
		parts = append(parts, fmt.Sprintf("%q:%q", k, v))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
