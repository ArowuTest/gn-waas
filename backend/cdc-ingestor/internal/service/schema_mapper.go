package service

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"os"
)

// SchemaMap holds the GWL → GN-WAAS column mapping configuration
type SchemaMap struct {
	GWLDatabase     GWLDatabaseConfig        `yaml:"gwl_database"`
	Tables          map[string]TableMapping  `yaml:"tables"`
	CategoryMapping map[string]string        `yaml:"category_mapping"`
}

// GWLDatabaseConfig holds GWL database connection details
type GWLDatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	Schema   string `yaml:"schema"`
	SSLMode  string `yaml:"ssl_mode"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// TableMapping holds the mapping for a single GWL table
type TableMapping struct {
	SourceTable string         `yaml:"source_table"`
	Enabled     bool           `yaml:"enabled"`
	Fields      []FieldMapping `yaml:"fields"`
}

// FieldMapping maps a GWL column to a GN-WAAS field
type FieldMapping struct {
	SourceColumn string `yaml:"source_column"`
	TargetField  string `yaml:"target_field"`
}

// SchemaMapper handles GWL → GN-WAAS data transformation
type SchemaMapper struct {
	schemaMap *SchemaMap
	logger    *zap.Logger
}

// NewSchemaMapper loads and validates the schema mapping configuration
func NewSchemaMapper(configPath string, logger *zap.Logger) (*SchemaMapper, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema map config: %w", err)
	}

	var schemaMap SchemaMap
	if err := yaml.Unmarshal(data, &schemaMap); err != nil {
		return nil, fmt.Errorf("failed to parse schema map config: %w", err)
	}

	logger.Info("Schema map loaded",
		zap.String("gwl_host", schemaMap.GWLDatabase.Host),
		zap.Int("tables", len(schemaMap.Tables)),
	)

	return &SchemaMapper{
		schemaMap: &schemaMap,
		logger:    logger,
	}, nil
}

// MapBillingRecord transforms a raw GWL billing row into GN-WAAS format
func (m *SchemaMapper) MapBillingRecord(rawRow map[string]interface{}) (*MappedBillingRecord, error) {
	tableMap, ok := m.schemaMap.Tables["billing"]
	if !ok {
		return nil, fmt.Errorf("billing table mapping not found")
	}

	if !tableMap.Enabled {
		return nil, fmt.Errorf("billing table mapping is disabled - configure gwl_schema_map.yaml")
	}

	mapped := &MappedBillingRecord{}
	fieldMap := buildFieldMap(tableMap.Fields)

	for gwlCol, gnwaasField := range fieldMap {
		val, exists := rawRow[gwlCol]
		if !exists {
			m.logger.Warn("GWL column not found in row",
				zap.String("column", gwlCol),
				zap.String("target", gnwaasField),
			)
			continue
		}

		if err := mapped.SetField(gnwaasField, val); err != nil {
			m.logger.Warn("Failed to set field",
				zap.String("field", gnwaasField),
				zap.Any("value", val),
				zap.Error(err),
			)
		}
	}

	// Normalise category
	if mapped.GWLCategory != "" {
		mapped.GWLCategory = m.normaliseCategory(mapped.GWLCategory)
	}

	return mapped, nil
}

// MapAccountRecord transforms a raw GWL account row into GN-WAAS format
func (m *SchemaMapper) MapAccountRecord(rawRow map[string]interface{}) (*MappedAccountRecord, error) {
	tableMap, ok := m.schemaMap.Tables["accounts"]
	if !ok {
		return nil, fmt.Errorf("accounts table mapping not found")
	}

	if !tableMap.Enabled {
		return nil, fmt.Errorf("accounts table mapping is disabled - configure gwl_schema_map.yaml")
	}

	mapped := &MappedAccountRecord{}
	fieldMap := buildFieldMap(tableMap.Fields)

	for gwlCol, gnwaasField := range fieldMap {
		val, exists := rawRow[gwlCol]
		if !exists {
			continue
		}
		_ = mapped.SetField(gnwaasField, val)
	}

	if mapped.Category != "" {
		mapped.Category = m.normaliseCategory(mapped.Category)
	}

	return mapped, nil
}

// normaliseCategory maps GWL category codes to GN-WAAS enums
func (m *SchemaMapper) normaliseCategory(gwlCategory string) string {
	upper := strings.ToUpper(strings.TrimSpace(gwlCategory))
	if mapped, ok := m.schemaMap.CategoryMapping[upper]; ok {
		return mapped
	}
	m.logger.Warn("Unknown GWL category code", zap.String("category", gwlCategory))
	return "RESIDENTIAL" // Safe default
}

// buildFieldMap creates a gwl_column → gnwaas_field lookup map
func buildFieldMap(fields []FieldMapping) map[string]string {
	m := make(map[string]string, len(fields))
	for _, f := range fields {
		m[f.SourceColumn] = f.TargetField
	}
	return m
}

// MappedBillingRecord holds a transformed GWL billing record
type MappedBillingRecord struct {
	GWLBillID          string
	GWLAccountNumber   string
	BillingPeriodStart time.Time
	BillingPeriodEnd   time.Time
	PreviousReading    float64
	CurrentReading     float64
	ConsumptionM3      float64
	GWLCategory        string
	GWLAmountGHS       float64
	GWLVatGHS          float64
	GWLTotalGHS        float64
	GWLReaderID        string
	GWLReadDate        time.Time
	GWLReadMethod      string
	PaymentStatus      string
	PaymentDate        *time.Time
	PaymentAmountGHS   float64
}

func (r *MappedBillingRecord) SetField(field string, val interface{}) error {
	switch field {
	case "gwl_bill_id":
		r.GWLBillID = toString(val)
	case "gwl_account_number":
		r.GWLAccountNumber = toString(val)
	case "billing_period_start":
		r.BillingPeriodStart = toTime(val)
	case "billing_period_end":
		r.BillingPeriodEnd = toTime(val)
	case "previous_reading":
		r.PreviousReading = toFloat64(val)
	case "current_reading":
		r.CurrentReading = toFloat64(val)
	case "consumption_m3":
		r.ConsumptionM3 = toFloat64(val)
	case "gwl_category":
		r.GWLCategory = toString(val)
	case "gwl_amount_ghs":
		r.GWLAmountGHS = toFloat64(val)
	case "gwl_vat_ghs":
		r.GWLVatGHS = toFloat64(val)
	case "gwl_total_ghs":
		r.GWLTotalGHS = toFloat64(val)
	case "gwl_reader_id":
		r.GWLReaderID = toString(val)
	case "gwl_read_date":
		r.GWLReadDate = toTime(val)
	case "gwl_read_method":
		r.GWLReadMethod = toString(val)
	case "payment_status":
		r.PaymentStatus = toString(val)
	case "payment_amount_ghs":
		r.PaymentAmountGHS = toFloat64(val)
	}
	return nil
}

// MappedAccountRecord holds a transformed GWL account record
type MappedAccountRecord struct {
	GWLAccountNumber string
	AccountHolderName string
	AccountHolderTIN  string
	Category          string
	Status            string
	DistrictCode      string
	MeterNumber       string
	MeterSerial       string
	MeterInstallDate  time.Time
	AddressLine1      string
	GPSLatitude       float64
	GPSLongitude      float64
	GWLBillingCycle   string
	GWLRouteCode      string
	GWLReaderID       string
}

func (r *MappedAccountRecord) SetField(field string, val interface{}) error {
	switch field {
	case "gwl_account_number":
		r.GWLAccountNumber = toString(val)
	case "account_holder_name":
		r.AccountHolderName = toString(val)
	case "account_holder_tin":
		r.AccountHolderTIN = toString(val)
	case "category":
		r.Category = toString(val)
	case "status":
		r.Status = toString(val)
	case "district_code":
		r.DistrictCode = toString(val)
	case "meter_number":
		r.MeterNumber = toString(val)
	case "meter_serial":
		r.MeterSerial = toString(val)
	case "meter_install_date":
		r.MeterInstallDate = toTime(val)
	case "address_line1":
		r.AddressLine1 = toString(val)
	case "gps_latitude":
		r.GPSLatitude = toFloat64(val)
	case "gps_longitude":
		r.GPSLongitude = toFloat64(val)
	case "gwl_billing_cycle":
		r.GWLBillingCycle = toString(val)
	case "gwl_route_code":
		r.GWLRouteCode = toString(val)
	case "gwl_reader_id":
		r.GWLReaderID = toString(val)
	}
	return nil
}


// IsGWLConfigured returns true if the GWL database host is configured
func (m *SchemaMapper) IsGWLConfigured() bool {
	return m.schemaMap.GWLDatabase.Host != ""
}

// ============================================================
// TYPE CONVERSION HELPERS
// ============================================================

func toString(val interface{}) string {
	if val == nil {
		return ""
	}
	return fmt.Sprintf("%v", val)
}

func toFloat64(val interface{}) float64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		var f float64
		fmt.Sscanf(v, "%f", &f)
		return f
	}
	return 0
}

func toTime(val interface{}) time.Time {
	if val == nil {
		return time.Time{}
	}
	switch v := val.(type) {
	case time.Time:
		return v
	case string:
		formats := []string{"2006-01-02", "2006-01-02 15:04:05", time.RFC3339}
		for _, f := range formats {
			if t, err := time.Parse(f, v); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}
