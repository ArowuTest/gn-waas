-- ============================================================
-- GN-WAAS Seed Data: 001_system_config
-- Description: Default system configuration values
--              All values are admin-configurable via the UI
-- ============================================================

INSERT INTO system_config (config_key, config_value, config_type, description, category) VALUES

-- Sentinel thresholds
('sentinel.shadow_bill_variance_pct',    '15.0',   'NUMBER',  'Shadow bill vs GWL actual variance threshold to trigger audit (%)', 'SENTINEL'),
('sentinel.night_flow_pct_of_daily',     '30.0',   'NUMBER',  'Night flow (2-4AM) as % of daily average to flag anomaly', 'SENTINEL'),
('sentinel.phantom_meter_months',        '6',      'NUMBER',  'Consecutive months of identical readings to flag phantom meter', 'SENTINEL'),
('sentinel.district_imbalance_pct',      '20.0',   'NUMBER',  'District production vs billing imbalance threshold (%)', 'SENTINEL'),
('sentinel.rationing_drop_pct',          '40.0',   'NUMBER',  'Expected consumption drop during rationing period (%)', 'SENTINEL'),
('sentinel.min_consumption_flag_m3',     '0.5',    'NUMBER',  'Minimum monthly consumption below which account is flagged', 'SENTINEL'),

-- Field operations
('field.gps_fence_radius_m',             '5.0',    'NUMBER',  'GPS geofence radius for meter reading (metres)', 'FIELD'),
('field.ocr_conflict_tolerance_pct',     '2.0',    'NUMBER',  'OCR vs manual reading tolerance before conflict alert (%)', 'FIELD'),
('field.max_photo_age_minutes',          '5',      'NUMBER',  'Maximum age of meter photo before rejection (minutes)', 'FIELD'),
('field.require_biometric',              'true',   'BOOLEAN', 'Require biometric verification for field officers', 'FIELD'),
('field.blind_audit_default',            'true',   'BOOLEAN', 'Enable blind audit mode by default (officer sees GPS only)', 'FIELD'),
('field.require_surroundings_photo',     'true',   'BOOLEAN', 'Require surroundings photo in addition to meter face photo', 'FIELD'),

-- GRA compliance
('gra.api_base_url',                     'https://vsdc.gra.gov.gh/api/v8.1', 'STRING', 'GRA VSDC API base URL', 'GRA'),
('gra.api_timeout_seconds',              '30',     'NUMBER',  'GRA API request timeout (seconds)', 'GRA'),
('gra.max_retry_attempts',               '3',      'NUMBER',  'Maximum GRA API retry attempts before marking FAILED', 'GRA'),
('gra.retry_delay_seconds',              '60',     'NUMBER',  'Delay between GRA API retry attempts (seconds)', 'GRA'),
('gra.vat_threshold_ghs',                '200.0',  'NUMBER',  'Minimum bill amount requiring GRA VAT signing (GHS)', 'GRA'),

-- Business model
('business.success_fee_rate_pct',        '3.0',    'NUMBER',  'Success fee rate on recovered revenue (%)', 'BUSINESS'),
('business.company_name',                'GN-WAAS Managed Services', 'STRING', 'Managing company name', 'BUSINESS'),
('business.company_tin',                 '',       'STRING',  'Managing company GRA TIN', 'BUSINESS'),

-- CDC synchronisation
('cdc.sync_interval_minutes',            '15',     'NUMBER',  'GWL database CDC sync interval (minutes)', 'CDC'),
('cdc.max_lag_minutes',                  '60',     'NUMBER',  'Maximum acceptable CDC lag before alert (minutes)', 'CDC'),
('cdc.batch_size',                       '1000',   'NUMBER',  'CDC sync batch size (records per batch)', 'CDC'),

-- Notifications
('notifications.anomaly_email_enabled',  'true',   'BOOLEAN', 'Send email notifications for new anomalies', 'NOTIFICATIONS'),
('notifications.critical_sms_enabled',   'true',   'BOOLEAN', 'Send SMS for CRITICAL level anomalies', 'NOTIFICATIONS'),
('notifications.sos_escalation_minutes', '5',      'NUMBER',  'Minutes before SOS auto-escalates to security', 'NOTIFICATIONS'),

-- System
('system.environment',                   'development', 'STRING', 'Deployment environment', 'SYSTEM'),
('system.pilot_district_code',           'TEMA-EAST',   'STRING', 'Active pilot district code', 'SYSTEM'),
('system.data_retention_years',          '7',      'NUMBER',  'Audit data retention period (years)', 'SYSTEM'),
('system.session_timeout_minutes',       '30',     'NUMBER',  'Admin portal session timeout (minutes)', 'SYSTEM');
