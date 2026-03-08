// ============================================================
// GN-WAAS Admin Portal — Realistic Mock Data
// ============================================================

export const MOCK_TOKEN = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbi11c2VyLTAwMSIsInJvbGUiOiJTWVNURU1fQURNSU4iLCJleHAiOjk5OTk5OTk5OTl9.mock'

export const MOCK_USER = {
  id: 'admin-user-001',
  email: 'admin@gnwaas.gov.gh',
  full_name: 'Kwame Asante',
  role: 'SYSTEM_ADMIN',
  district_id: null,
  is_active: true,
  is_mfa_enabled: true,
  created_at: '2026-01-01T00:00:00Z',
}

export const MOCK_DISTRICTS = [
  { id: 'dist-001', district_name: 'Accra Metropolitan', region: 'Greater Accra', zone_type: 'URBAN', gps_latitude: 5.6037, gps_longitude: -0.1870, is_pilot_district: true, loss_ratio_pct: 48.2 },
  { id: 'dist-002', district_name: 'Kumasi Metropolitan', region: 'Ashanti', zone_type: 'URBAN', gps_latitude: 6.6885, gps_longitude: -1.6244, is_pilot_district: true, loss_ratio_pct: 52.1 },
  { id: 'dist-003', district_name: 'Tema Municipal', region: 'Greater Accra', zone_type: 'PERI_URBAN', gps_latitude: 5.6698, gps_longitude: 0.0166, is_pilot_district: false, loss_ratio_pct: 44.7 },
  { id: 'dist-004', district_name: 'Tamale Metropolitan', region: 'Northern', zone_type: 'URBAN', gps_latitude: 9.4008, gps_longitude: -0.8393, is_pilot_district: false, loss_ratio_pct: 61.3 },
  { id: 'dist-005', district_name: 'Cape Coast Municipal', region: 'Central', zone_type: 'PERI_URBAN', gps_latitude: 5.1053, gps_longitude: -1.2466, is_pilot_district: false, loss_ratio_pct: 55.8 },
]

export const MOCK_DASHBOARD_STATS = {
  pending: 47,
  in_progress: 23,
  total_confirmed_loss_ghs: 1842650.00,
  gra_signed: 312,
  total_success_fees_ghs: 55279.50,
  completed: 891,
  total: 1274,
  pending_assignment: 31,
}

export const MOCK_ANOMALY_FLAGS = [
  { id: 'flag-001', flag_reference: 'ANF-2026-0847', district_id: 'dist-001', district_name: 'Accra Metropolitan', account_id: 'acc-001', account_number: 'GWL-ACC-004821', customer_name: 'Kofi Mensah Enterprises', anomaly_type: 'SHADOW_BILL_VARIANCE', alert_level: 'CRITICAL', status: 'OPEN', estimated_loss_ghs: 48200.00, monthly_leakage_ghs: 48200.00, annualised_leakage_ghs: 578400.00, leakage_category: 'REVENUE_LEAKAGE', variance_pct: 67.4, created_at: '2026-03-05T08:14:22Z', updated_at: '2026-03-05T08:14:22Z' },
  { id: 'flag-002', flag_reference: 'ANF-2026-0846', district_id: 'dist-002', district_name: 'Kumasi Metropolitan', account_id: 'acc-002', account_number: 'GWL-KSI-002341', customer_name: 'Ama Boateng Cold Store', anomaly_type: 'UNAUTHORISED_CONSUMPTION', alert_level: 'HIGH', status: 'OPEN', estimated_loss_ghs: 22100.00, monthly_leakage_ghs: 22100.00, annualised_leakage_ghs: 265200.00, leakage_category: 'REVENUE_LEAKAGE', variance_pct: 41.2, created_at: '2026-03-05T07:30:11Z', updated_at: '2026-03-05T07:30:11Z' },
  { id: 'flag-003', flag_reference: 'ANF-2026-0845', district_id: 'dist-001', district_name: 'Accra Metropolitan', account_id: 'acc-003', account_number: 'GWL-ACC-007712', customer_name: 'Tema Industrial Park Ltd', anomaly_type: 'NIGHT_FLOW_ANOMALY', alert_level: 'HIGH', status: 'ACKNOWLEDGED', estimated_loss_ghs: 18750.00, monthly_leakage_ghs: 18750.00, annualised_leakage_ghs: 225000.00, leakage_category: 'REVENUE_LEAKAGE', variance_pct: 38.9, created_at: '2026-03-04T22:15:44Z', updated_at: '2026-03-05T06:00:00Z' },
  { id: 'flag-004', flag_reference: 'ANF-2026-0844', district_id: 'dist-003', district_name: 'Tema Municipal', account_id: 'acc-004', account_number: 'GWL-TEM-001198', customer_name: 'Harbour View Hotel', anomaly_type: 'ADDRESS_UNVERIFIED', alert_level: 'HIGH', status: 'OPEN', estimated_loss_ghs: 15400.00, monthly_leakage_ghs: null, annualised_leakage_ghs: null, leakage_category: 'DATA_QUALITY', variance_pct: 29.7, created_at: '2026-03-04T18:42:33Z', updated_at: '2026-03-04T18:42:33Z' },
  { id: 'flag-005', flag_reference: 'ANF-2026-0843', district_id: 'dist-002', district_name: 'Kumasi Metropolitan', account_id: 'acc-005', account_number: 'GWL-KSI-005567', customer_name: 'Ashanti Breweries Depot', anomaly_type: 'CATEGORY_MISMATCH', alert_level: 'MEDIUM', status: 'OPEN', estimated_loss_ghs: 9800.00, monthly_leakage_ghs: 9800.00, annualised_leakage_ghs: 117600.00, leakage_category: 'REVENUE_LEAKAGE', variance_pct: 22.1, created_at: '2026-03-04T14:20:18Z', updated_at: '2026-03-04T14:20:18Z' },
  { id: 'flag-006', flag_reference: 'ANF-2026-0842', district_id: 'dist-004', district_name: 'Tamale Metropolitan', account_id: 'acc-006', account_number: 'GWL-TAM-000892', customer_name: 'Northern Star Filling Station', anomaly_type: 'ADDRESS_UNVERIFIED', alert_level: 'MEDIUM', status: 'RESOLVED', estimated_loss_ghs: 7200.00, monthly_leakage_ghs: null, annualised_leakage_ghs: null, leakage_category: 'DATA_QUALITY', variance_pct: 18.4, created_at: '2026-03-03T10:11:05Z', updated_at: '2026-03-05T09:00:00Z' },
  { id: 'flag-007', flag_reference: 'ANF-2026-0841', district_id: 'dist-001', district_name: 'Accra Metropolitan', account_id: 'acc-007', account_number: 'GWL-ACC-012334', customer_name: 'Osu Castle Apartments', anomaly_type: 'METERING_INACCURACY', alert_level: 'MEDIUM', status: 'OPEN', estimated_loss_ghs: 6100.00, monthly_leakage_ghs: 6100.00, annualised_leakage_ghs: 73200.00, leakage_category: 'REVENUE_LEAKAGE', variance_pct: 16.8, created_at: '2026-03-03T08:55:30Z', updated_at: '2026-03-03T08:55:30Z' },
  { id: 'flag-008', flag_reference: 'ANF-2026-0840', district_id: 'dist-005', district_name: 'Cape Coast Municipal', account_id: 'acc-008', account_number: 'GWL-CAP-003341', customer_name: 'Cape Coast University Hostel', anomaly_type: 'DISTRICT_IMBALANCE', alert_level: 'LOW', status: 'OPEN', estimated_loss_ghs: 3400.00, monthly_leakage_ghs: 3400.00, annualised_leakage_ghs: 40800.00, leakage_category: 'REVENUE_LEAKAGE', variance_pct: 12.3, created_at: '2026-03-02T16:30:00Z', updated_at: '2026-03-02T16:30:00Z' },
]

export const MOCK_AUDIT_EVENTS = [
  { id: 'audit-001', audit_reference: 'AUD-2026-1247', account_id: 'acc-001', account_number: 'GWL-ACC-004821', account_holder: 'Kofi Mensah Enterprises', district_id: 'dist-001', district_name: 'Accra Metropolitan', status: 'IN_PROGRESS', gra_status: 'PENDING', gwl_billed_ghs: 71400.00, shadow_bill_ghs: 119600.00, variance_pct: 67.4, confirmed_loss_ghs: 48200.00, success_fee_ghs: 1446.00, gra_sdc_id: null, gra_qr_code_url: null, gra_signed_at: null, is_locked: false, created_at: '2026-03-05T08:14:22Z', updated_at: '2026-03-05T10:30:00Z' },
  { id: 'audit-002', audit_reference: 'AUD-2026-1246', account_id: 'acc-002', account_number: 'GWL-KSI-002341', account_holder: 'Ama Boateng Cold Store', district_id: 'dist-002', district_name: 'Kumasi Metropolitan', status: 'AWAITING_GRA', gra_status: 'PENDING', gwl_billed_ghs: 53700.00, shadow_bill_ghs: 75800.00, variance_pct: 41.2, confirmed_loss_ghs: 22100.00, success_fee_ghs: 663.00, gra_sdc_id: null, gra_qr_code_url: null, gra_signed_at: null, is_locked: false, created_at: '2026-03-05T07:30:11Z', updated_at: '2026-03-05T09:15:00Z' },
  { id: 'audit-003', audit_reference: 'AUD-2026-1245', account_id: 'acc-009', account_number: 'GWL-ACC-009981', account_holder: 'Accra Mall Management', district_id: 'dist-001', district_name: 'Accra Metropolitan', status: 'GRA_CONFIRMED', gra_status: 'SIGNED', gwl_billed_ghs: 142000.00, shadow_bill_ghs: 198400.00, variance_pct: 39.7, confirmed_loss_ghs: 56400.00, success_fee_ghs: 1692.00, gra_sdc_id: 'GRA-SDC-2026-004421', gra_qr_code_url: 'https://gra.gov.gh/vsdc/qr/004421', gra_signed_at: '2026-03-04T16:22:00Z', is_locked: true, created_at: '2026-03-03T11:00:00Z', updated_at: '2026-03-04T16:22:00Z' },
  { id: 'audit-004', audit_reference: 'AUD-2026-1244', account_id: 'acc-010', account_number: 'GWL-TEM-004412', account_holder: 'Meridian Shipping Co.', district_id: 'dist-003', district_name: 'Tema Municipal', status: 'COMPLETED', gra_status: 'SIGNED', gwl_billed_ghs: 88200.00, shadow_bill_ghs: 121000.00, variance_pct: 37.2, confirmed_loss_ghs: 32800.00, success_fee_ghs: 984.00, gra_sdc_id: 'GRA-SDC-2026-004418', gra_qr_code_url: 'https://gra.gov.gh/vsdc/qr/004418', gra_signed_at: '2026-03-03T14:10:00Z', is_locked: true, created_at: '2026-03-02T09:00:00Z', updated_at: '2026-03-03T14:10:00Z' },
  { id: 'audit-005', audit_reference: 'AUD-2026-1243', account_id: 'acc-011', account_number: 'GWL-KSI-007823', account_holder: 'Kumasi Central Market Assoc.', district_id: 'dist-002', district_name: 'Kumasi Metropolitan', status: 'PENDING', gra_status: 'PENDING', gwl_billed_ghs: 34100.00, shadow_bill_ghs: 46200.00, variance_pct: 35.5, confirmed_loss_ghs: null, success_fee_ghs: null, gra_sdc_id: null, gra_qr_code_url: null, gra_signed_at: null, is_locked: false, created_at: '2026-03-05T06:00:00Z', updated_at: '2026-03-05T06:00:00Z' },
]

export const MOCK_FIELD_JOBS = [
  { id: 'job-001', job_reference: 'FJ-2026-0412', audit_event_id: 'audit-001', district_id: 'dist-001', district_name: 'Accra Metropolitan', account_number: 'GWL-ACC-004821', customer_name: 'Kofi Mensah Enterprises', address: '14 Ring Road East, Accra', status: 'ON_SITE', alert_level: 'CRITICAL', assigned_officer_id: 'officer-001', officer_name: 'Abena Owusu', target_gps_lat: 5.6037, target_gps_lng: -0.1870, created_at: '2026-03-05T09:00:00Z', updated_at: '2026-03-05T10:45:00Z' },
  { id: 'job-002', job_reference: 'FJ-2026-0411', audit_event_id: 'audit-002', district_id: 'dist-002', district_name: 'Kumasi Metropolitan', account_number: 'GWL-KSI-002341', customer_name: 'Ama Boateng Cold Store', address: '7 Adum Road, Kumasi', status: 'EN_ROUTE', alert_level: 'HIGH', assigned_officer_id: 'officer-002', officer_name: 'Kweku Darko', target_gps_lat: 6.6885, target_gps_lng: -1.6244, created_at: '2026-03-05T08:30:00Z', updated_at: '2026-03-05T10:20:00Z' },
  { id: 'job-003', job_reference: 'FJ-2026-0410', audit_event_id: null, district_id: 'dist-003', district_name: 'Tema Municipal', account_number: 'GWL-TEM-001198', customer_name: 'Harbour View Hotel', address: '22 Harbour Road, Tema', status: 'QUEUED', alert_level: 'HIGH', assigned_officer_id: null, officer_name: null, target_gps_lat: 5.6698, target_gps_lng: 0.0166, created_at: '2026-03-05T07:00:00Z', updated_at: '2026-03-05T07:00:00Z' },
  { id: 'job-004', job_reference: 'FJ-2026-0409', audit_event_id: 'audit-004', district_id: 'dist-003', district_name: 'Tema Municipal', account_number: 'GWL-TEM-004412', customer_name: 'Meridian Shipping Co.', address: '5 Port Access Road, Tema', status: 'COMPLETED', alert_level: 'HIGH', assigned_officer_id: 'officer-003', officer_name: 'Yaa Asantewaa', target_gps_lat: 5.6720, target_gps_lng: 0.0180, created_at: '2026-03-02T08:00:00Z', updated_at: '2026-03-03T13:00:00Z' },
  { id: 'job-005', job_reference: 'FJ-2026-0408', audit_event_id: null, district_id: 'dist-004', district_name: 'Tamale Metropolitan', account_number: 'GWL-TAM-000892', customer_name: 'Northern Star Filling Station', address: '3 Bolgatanga Road, Tamale', status: 'SOS', alert_level: 'CRITICAL', assigned_officer_id: 'officer-004', officer_name: 'Ibrahim Mahama', target_gps_lat: 9.4008, target_gps_lng: -0.8393, created_at: '2026-03-05T06:00:00Z', updated_at: '2026-03-05T11:00:00Z' },
]

export const MOCK_WATER_BALANCE = {
  period_start: '2026-03-01T00:00:00Z',
  period_end: '2026-03-05T23:59:59Z',
  system_input_m3: 4820000,
  authorized_consumption_m3: 2340000,
  billed_authorized_m3: 2180000,
  unbilled_authorized_m3: 160000,
  water_losses_m3: 2480000,
  apparent_losses_m3: 1420000,
  real_losses_m3: 1060000,
  nrw_m3: 2480000,
  nrw_pct: 51.45,
  real_loss_value_ghs: 892400.00,
  total_nrw_value_ghs: 2084800.00,
  district_id: null,
  created_at: '2026-03-06T00:00:00Z',
}

export const MOCK_REVENUE_SUMMARY = {
  total_events: 1274,
  total_variance_ghs: 8420000.00,
  total_recovered_ghs: 1842650.00,
  total_success_fee_ghs: 55279.50,
  pending_count: 47,
  confirmed_count: 312,
  collected_count: 891,
  by_type: [
    { recovery_type: 'SHADOW_BILL_VARIANCE', count: 612, recovered_ghs: 980000.00, success_fee_ghs: 29400.00 },
    { recovery_type: 'UNAUTHORISED_CONSUMPTION', count: 341, recovered_ghs: 542000.00, success_fee_ghs: 16260.00 },
    { recovery_type: 'PHANTOM_METER', count: 198, recovered_ghs: 220000.00, success_fee_ghs: 6600.00 },
    { recovery_type: 'ADDRESS_UNVERIFIED', count: 123, recovered_ghs: 100650.00, success_fee_ghs: 3019.50 },
  ],
}

export const MOCK_WORKFORCE_SUMMARY = {
  total_officers: 48,
  active_officers: 31,
  on_field: 12,
  available: 19,
  jobs_today: 23,
  jobs_completed_today: 14,
  avg_completion_time_hrs: 2.4,
}

export const MOCK_ACTIVE_OFFICERS = [
  { id: 'officer-001', full_name: 'Abena Owusu', employee_id: 'EMP-0041', role: 'FIELD_OFFICER', status: 'ON_FIELD', district_name: 'Accra Metropolitan', current_job: 'FJ-2026-0412', last_seen: '2026-03-06T05:55:00Z' },
  { id: 'officer-002', full_name: 'Kweku Darko', employee_id: 'EMP-0038', role: 'FIELD_OFFICER', status: 'ON_FIELD', district_name: 'Kumasi Metropolitan', current_job: 'FJ-2026-0411', last_seen: '2026-03-06T05:50:00Z' },
  { id: 'officer-003', full_name: 'Yaa Asantewaa', employee_id: 'EMP-0029', role: 'FIELD_OFFICER', status: 'AVAILABLE', district_name: 'Tema Municipal', current_job: null, last_seen: '2026-03-06T05:30:00Z' },
  { id: 'officer-004', full_name: 'Ibrahim Mahama', employee_id: 'EMP-0052', role: 'FIELD_OFFICER', status: 'SOS', district_name: 'Tamale Metropolitan', current_job: 'FJ-2026-0408', last_seen: '2026-03-06T05:58:00Z' },
  { id: 'officer-005', full_name: 'Efua Asare', employee_id: 'EMP-0033', role: 'FIELD_SUPERVISOR', status: 'AVAILABLE', district_name: 'Accra Metropolitan', current_job: null, last_seen: '2026-03-06T05:45:00Z' },
  { id: 'officer-006', full_name: 'Nana Acheampong', employee_id: 'EMP-0047', role: 'FIELD_OFFICER', status: 'ON_FIELD', district_name: 'Cape Coast Municipal', current_job: 'FJ-2026-0407', last_seen: '2026-03-06T05:52:00Z' },
]

export const MOCK_NRW_SUMMARY = [
  { district_id: 'dist-001', district_name: 'Accra Metropolitan', region: 'Greater Accra', zone_type: 'URBAN', system_input_m3: 1820000, billed_m3: 944000, nrw_m3: 876000, nrw_pct: 48.1, apparent_loss_m3: 520000, real_loss_m3: 356000, loss_ratio_pct: 48.1, data_confidence_grade: 92, is_pilot_district: true, grade: 'D' },
  { district_id: 'dist-002', district_name: 'Kumasi Metropolitan', region: 'Ashanti', zone_type: 'URBAN', system_input_m3: 1540000, billed_m3: 738000, nrw_m3: 802000, nrw_pct: 52.1, apparent_loss_m3: 480000, real_loss_m3: 322000, loss_ratio_pct: 52.1, data_confidence_grade: 88, is_pilot_district: true, grade: 'F' },
  { district_id: 'dist-003', district_name: 'Tema Municipal', region: 'Greater Accra', zone_type: 'PERI_URBAN', system_input_m3: 680000, billed_m3: 376000, nrw_m3: 304000, nrw_pct: 44.7, apparent_loss_m3: 180000, real_loss_m3: 124000, loss_ratio_pct: 44.7, data_confidence_grade: 85, is_pilot_district: false, grade: 'D' },
  { district_id: 'dist-004', district_name: 'Tamale Metropolitan', region: 'Northern', zone_type: 'URBAN', system_input_m3: 520000, billed_m3: 201000, nrw_m3: 319000, nrw_pct: 61.3, apparent_loss_m3: 190000, real_loss_m3: 129000, loss_ratio_pct: 61.3, data_confidence_grade: 71, is_pilot_district: false, grade: 'F' },
  { district_id: 'dist-005', district_name: 'Cape Coast Municipal', region: 'Central', zone_type: 'PERI_URBAN', system_input_m3: 260000, billed_m3: 115000, nrw_m3: 145000, nrw_pct: 55.8, apparent_loss_m3: 88000, real_loss_m3: 57000, loss_ratio_pct: 55.8, data_confidence_grade: 79, is_pilot_district: false, grade: 'F' },
]

export const MOCK_USERS = [
  { id: 'admin-user-001', email: 'admin@gnwaas.gov.gh', full_name: 'Kwame Asante', role: 'SYSTEM_ADMIN', district_id: null, district_name: null, is_active: true, is_mfa_enabled: true, employee_id: 'SYS-001', created_at: '2026-01-01T00:00:00Z' },
  { id: 'mgr-user-001', email: 'audit.manager@gnwaas.gov.gh', full_name: 'Akosua Frimpong', role: 'AUDIT_MANAGER', district_id: 'dist-001', district_name: 'Accra Metropolitan', is_active: true, is_mfa_enabled: true, employee_id: 'MGR-001', created_at: '2026-01-15T00:00:00Z' },
  { id: 'gra-user-001', email: 'gra.auditor@gra.gov.gh', full_name: 'Kofi Boateng', role: 'GRA_AUDITOR', district_id: null, district_name: null, is_active: true, is_mfa_enabled: true, employee_id: 'GRA-001', created_at: '2026-01-20T00:00:00Z' },
  { id: 'gwl-user-001', email: 'gwl.mgmt@gwl.com.gh', full_name: 'Ama Sarpong', role: 'GWL_MANAGEMENT', district_id: null, district_name: null, is_active: true, is_mfa_enabled: false, employee_id: 'GWL-001', created_at: '2026-02-01T00:00:00Z' },
  { id: 'sup-user-001', email: 'supervisor@gnwaas.gov.gh', full_name: 'Efua Asare', role: 'FIELD_SUPERVISOR', district_id: 'dist-001', district_name: 'Accra Metropolitan', is_active: true, is_mfa_enabled: false, employee_id: 'EMP-0033', created_at: '2026-02-10T00:00:00Z' },
  ...MOCK_ACTIVE_OFFICERS.map(o => ({ id: o.id, email: `${o.full_name.toLowerCase().replace(' ', '.')}@gnwaas.gov.gh`, full_name: o.full_name, role: o.role, district_id: 'dist-001', district_name: o.district_name, is_active: true, is_mfa_enabled: false, employee_id: o.employee_id, created_at: '2026-02-15T00:00:00Z' })),
]

export const MOCK_SYSTEM_CONFIG = [
  { id: 'cfg-001', category: 'sentinel', key: 'sentinel.shadow_bill_variance_pct', value: '15', description: 'Shadow bill variance threshold (%)', updated_at: '2026-03-01T00:00:00Z' },
  { id: 'cfg-002', category: 'sentinel', key: 'sentinel.night_flow_threshold_m3h', value: '2.5', description: 'Night flow anomaly threshold (m³/h)', updated_at: '2026-03-01T00:00:00Z' },
  { id: 'cfg-003', category: 'sentinel', key: 'sentinel.gps_lock_radius_m', value: '50', description: 'GPS lock radius for field verification (m)', updated_at: '2026-03-01T00:00:00Z' },
  { id: 'cfg-004', category: 'tariff', key: 'tariff.residential_tier1_m3', value: '5', description: 'Residential tier 1 upper limit (m³)', updated_at: '2026-03-01T00:00:00Z' },
  { id: 'cfg-005', category: 'tariff', key: 'tariff.residential_tier1_rate', value: '6.1225', description: 'Residential tier 1 rate (₵/m³)', updated_at: '2026-03-01T00:00:00Z' },
  { id: 'cfg-006', category: 'tariff', key: 'tariff.residential_tier2_rate', value: '10.8320', description: 'Residential tier 2 rate (₵/m³)', updated_at: '2026-03-01T00:00:00Z' },
  { id: 'cfg-007', category: 'tariff', key: 'tariff.commercial_fixed_ghs', value: '25000', description: 'Commercial fixed charge (₵)', updated_at: '2026-03-01T00:00:00Z' },
  { id: 'cfg-008', category: 'tariff', key: 'tariff.vat_rate', value: '0.20', description: 'VAT rate (20%)', updated_at: '2026-03-01T00:00:00Z' },
]
