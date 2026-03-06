import { http, HttpResponse, delay } from 'msw'

const BASE = '/api/v1'
const d = (ms = 180) => delay(ms)

// ─── Shared mock data ────────────────────────────────────────────────────────

export const MOCK_TOKEN = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbi0wMDEiLCJyb2xlIjoiU1lTVEVNX0FETUlOIiwiZXhwIjo5OTk5OTk5OTk5fQ.mock'

export const MOCK_USER = {
  id: 'admin-001',
  email: 'admin@gnwaas.gov.gh',
  full_name: 'Kwame Asante',
  role: 'SYSTEM_ADMIN',
  district_id: null,
  is_active: true,
  employee_id: 'EMP-0001',
}

export const MOCK_DISTRICTS = [
  { id: 'dist-001', district_name: 'Accra Metropolitan', region: 'Greater Accra', zone_type: 'URBAN', geographic_zone: 'SOUTHERN', gps_latitude: 5.6037, gps_longitude: -0.1870, is_pilot_district: true, loss_ratio_pct: 48.2 },
  { id: 'dist-002', district_name: 'Kumasi Metropolitan', region: 'Ashanti', zone_type: 'URBAN', geographic_zone: 'MIDDLE', gps_latitude: 6.6885, gps_longitude: -1.6244, is_pilot_district: true, loss_ratio_pct: 52.1 },
  { id: 'dist-003', district_name: 'Tema Municipal', region: 'Greater Accra', zone_type: 'PERI_URBAN', geographic_zone: 'SOUTHERN', gps_latitude: 5.6698, gps_longitude: 0.0166, is_pilot_district: false, loss_ratio_pct: 44.7 },
  { id: 'dist-004', district_name: 'Tamale Metropolitan', region: 'Northern', zone_type: 'URBAN', geographic_zone: 'NORTHERN', gps_latitude: 9.4008, gps_longitude: -0.8393, is_pilot_district: false, loss_ratio_pct: 61.3 },
  { id: 'dist-005', district_name: 'Cape Coast Municipal', region: 'Central', zone_type: 'PERI_URBAN', geographic_zone: 'SOUTHERN', gps_latitude: 5.1053, gps_longitude: -1.2466, is_pilot_district: false, loss_ratio_pct: 55.8 },
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

// Water balance — array of WaterBalanceRecord (one per district per period)
export const MOCK_WATER_BALANCE: object[] = [
  {
    district_id: 'dist-001',
    period_start: '2026-03-01T00:00:00Z',
    period_end: '2026-03-05T23:59:59Z',
    system_input_m3: 1820000,
    billed_metered_m3: 944000,
    billed_unmetered_m3: 12000,
    unbilled_metered_m3: 38000,
    unbilled_unmetered_m3: 8000,
    total_authorised_m3: 1002000,
    unauthorised_consumption_m3: 520000,
    metering_inaccuracies_m3: 148000,
    data_handling_errors_m3: 24000,
    total_apparent_losses_m3: 692000,
    main_leakage_m3: 82000,
    storage_overflow_m3: 18000,
    service_connection_leak_m3: 26000,
    total_real_losses_m3: 126000,
    total_water_losses_m3: 818000,
    nrw_m3: 876000,
    nrw_percent: 48.1,
    ili: 3.2,
    iwa_grade: 'D',
    estimated_revenue_recovery_ghs: 1842650.00,
    data_confidence_score: 92,
    computed_at: '2026-03-06T05:00:00Z',
  },
]

export const MOCK_ANOMALY_FLAGS = [
  { id: 'flag-001', flag_reference: 'ANF-2026-0847', district_id: 'dist-001', district_name: 'Accra Metropolitan', account_number: 'GWL-ACC-004821', customer_name: 'Kofi Mensah Enterprises', anomaly_type: 'SHADOW_BILL_VARIANCE', alert_level: 'CRITICAL', status: 'OPEN', estimated_loss_ghs: 48200.00, variance_pct: 67.4, created_at: '2026-03-05T08:14:22Z', updated_at: '2026-03-05T10:30:00Z' },
  { id: 'flag-002', flag_reference: 'ANF-2026-0846', district_id: 'dist-001', district_name: 'Accra Metropolitan', account_number: 'GWL-ACC-007712', customer_name: 'Tema Industrial Park Ltd', anomaly_type: 'NIGHT_FLOW_ANOMALY', alert_level: 'HIGH', status: 'OPEN', estimated_loss_ghs: 18750.00, variance_pct: 38.9, created_at: '2026-03-04T22:15:44Z', updated_at: '2026-03-05T06:00:00Z' },
  { id: 'flag-003', flag_reference: 'ANF-2026-0845', district_id: 'dist-002', district_name: 'Kumasi Metropolitan', account_number: 'GWL-KSI-002341', customer_name: 'Ama Boateng Cold Store', anomaly_type: 'UNAUTHORISED_CONSUMPTION', alert_level: 'HIGH', status: 'ACKNOWLEDGED', estimated_loss_ghs: 22100.00, variance_pct: 41.2, created_at: '2026-03-04T14:00:00Z', updated_at: '2026-03-05T09:00:00Z' },
  { id: 'flag-004', flag_reference: 'ANF-2026-0844', district_id: 'dist-003', district_name: 'Tema Municipal', account_number: 'GWL-TEM-004412', customer_name: 'Meridian Shipping Co.', anomaly_type: 'PHANTOM_METER', alert_level: 'HIGH', status: 'OPEN', estimated_loss_ghs: 32800.00, variance_pct: 37.2, created_at: '2026-03-03T09:00:00Z', updated_at: '2026-03-04T11:00:00Z' },
  { id: 'flag-005', flag_reference: 'ANF-2026-0843', district_id: 'dist-002', district_name: 'Kumasi Metropolitan', account_number: 'GWL-KSI-007823', customer_name: 'Kumasi Central Market Assoc.', anomaly_type: 'CATEGORY_MISMATCH', alert_level: 'MEDIUM', status: 'RESOLVED', estimated_loss_ghs: 12100.00, variance_pct: 35.5, created_at: '2026-03-02T06:00:00Z', updated_at: '2026-03-03T14:00:00Z' },
  { id: 'flag-006', flag_reference: 'ANF-2026-0842', district_id: 'dist-004', district_name: 'Tamale Metropolitan', account_number: 'GWL-TAM-000892', customer_name: 'Northern Star Filling Station', anomaly_type: 'GHOST_ACCOUNT', alert_level: 'MEDIUM', status: 'OPEN', estimated_loss_ghs: 9200.00, variance_pct: 32.4, created_at: '2026-03-01T14:00:00Z', updated_at: '2026-03-02T08:00:00Z' },
  { id: 'flag-007', flag_reference: 'ANF-2026-0841', district_id: 'dist-001', district_name: 'Accra Metropolitan', account_number: 'GWL-ACC-012334', customer_name: 'Osu Castle Apartments', anomaly_type: 'METERING_INACCURACY', alert_level: 'MEDIUM', status: 'RESOLVED', estimated_loss_ghs: 6100.00, variance_pct: 16.8, created_at: '2026-02-28T08:55:30Z', updated_at: '2026-03-01T12:00:00Z' },
  { id: 'flag-008', flag_reference: 'ANF-2026-0840', district_id: 'dist-005', district_name: 'Cape Coast Municipal', account_number: 'GWL-CC-003312', customer_name: 'Cape Coast Polytechnic', anomaly_type: 'BILLING_VARIANCE', alert_level: 'LOW', status: 'OPEN', estimated_loss_ghs: 4400.00, variance_pct: 22.1, created_at: '2026-02-27T10:00:00Z', updated_at: '2026-02-27T10:00:00Z' },
]

export const MOCK_AUDIT_EVENTS = [
  { id: 'audit-001', audit_reference: 'AUD-2026-1247', account_number: 'GWL-ACC-004821', account_holder: 'Kofi Mensah Enterprises', district_id: 'dist-001', district_name: 'Accra Metropolitan', status: 'IN_PROGRESS', gra_status: 'PENDING', gwl_billed_ghs: 71400.00, shadow_bill_ghs: 119600.00, variance_pct: 67.4, confirmed_loss_ghs: null, success_fee_ghs: null, gra_receipt_number: null, gra_qr_code: null, gra_locked_at: null, created_at: '2026-03-05T08:14:22Z', updated_at: '2026-03-05T10:30:00Z' },
  { id: 'audit-002', audit_reference: 'AUD-2026-1246', account_number: 'GWL-KSI-002341', account_holder: 'Ama Boateng Cold Store', district_id: 'dist-002', district_name: 'Kumasi Metropolitan', status: 'AWAITING_GRA', gra_status: 'PENDING', gwl_billed_ghs: 53700.00, shadow_bill_ghs: 75800.00, variance_pct: 41.2, confirmed_loss_ghs: 22100.00, success_fee_ghs: 663.00, gra_receipt_number: null, gra_qr_code: null, gra_locked_at: null, created_at: '2026-03-05T07:30:11Z', updated_at: '2026-03-05T09:15:00Z' },
  { id: 'audit-003', audit_reference: 'AUD-2026-1245', account_number: 'GWL-ACC-009981', account_holder: 'Accra Mall Management', district_id: 'dist-001', district_name: 'Accra Metropolitan', status: 'GRA_CONFIRMED', gra_status: 'CONFIRMED', gwl_billed_ghs: 142000.00, shadow_bill_ghs: 198400.00, variance_pct: 39.7, confirmed_loss_ghs: 56400.00, success_fee_ghs: 1692.00, gra_receipt_number: 'GRA-2026-004821', gra_qr_code: 'https://verify.gra.gov.gh/qr/GRA-2026-004821', gra_locked_at: '2026-03-04T16:22:00Z', created_at: '2026-03-03T11:00:00Z', updated_at: '2026-03-04T16:22:00Z' },
  { id: 'audit-004', audit_reference: 'AUD-2026-1244', account_number: 'GWL-TEM-004412', account_holder: 'Meridian Shipping Co.', district_id: 'dist-003', district_name: 'Tema Municipal', status: 'COMPLETED', gra_status: 'CONFIRMED', gwl_billed_ghs: 88200.00, shadow_bill_ghs: 121000.00, variance_pct: 37.2, confirmed_loss_ghs: 32800.00, success_fee_ghs: 984.00, gra_receipt_number: 'GRA-2026-004412', gra_qr_code: 'https://verify.gra.gov.gh/qr/GRA-2026-004412', gra_locked_at: '2026-03-03T14:10:00Z', created_at: '2026-03-02T09:00:00Z', updated_at: '2026-03-03T14:10:00Z' },
  { id: 'audit-005', audit_reference: 'AUD-2026-1243', account_number: 'GWL-KSI-007823', account_holder: 'Kumasi Central Market Assoc.', district_id: 'dist-002', district_name: 'Kumasi Metropolitan', status: 'PENDING', gra_status: 'NOT_SUBMITTED', gwl_billed_ghs: 34100.00, shadow_bill_ghs: 46200.00, variance_pct: 35.5, confirmed_loss_ghs: null, success_fee_ghs: null, gra_receipt_number: null, gra_qr_code: null, gra_locked_at: null, created_at: '2026-03-05T06:00:00Z', updated_at: '2026-03-05T06:00:00Z' },
]

export const MOCK_FIELD_JOBS = [
  { id: 'job-001', job_reference: 'FJ-2026-0412', district_id: 'dist-001', district_name: 'Accra Metropolitan', account_number: 'GWL-ACC-004821', customer_name: 'Kofi Mensah Enterprises', address: '14 Ring Road East, Accra', status: 'ON_SITE', alert_level: 'CRITICAL', assigned_officer_id: 'officer-001', officer_name: 'Abena Owusu', target_gps_lat: 5.6037, target_gps_lng: -0.1870, created_at: '2026-03-05T09:00:00Z', updated_at: '2026-03-05T10:45:00Z' },
  { id: 'job-002', job_reference: 'FJ-2026-0411', district_id: 'dist-001', district_name: 'Accra Metropolitan', account_number: 'GWL-ACC-007712', customer_name: 'Tema Industrial Park Ltd', address: '22 Industrial Ave, Tema', status: 'QUEUED', alert_level: 'HIGH', assigned_officer_id: null, officer_name: null, target_gps_lat: 5.6698, target_gps_lng: 0.0166, created_at: '2026-03-05T08:00:00Z', updated_at: '2026-03-05T08:00:00Z' },
  { id: 'job-003', job_reference: 'FJ-2026-0410', district_id: 'dist-002', district_name: 'Kumasi Metropolitan', account_number: 'GWL-KSI-002341', customer_name: 'Ama Boateng Cold Store', address: '7 Adum Road, Kumasi', status: 'DISPATCHED', alert_level: 'HIGH', assigned_officer_id: 'officer-002', officer_name: 'Kweku Darko', target_gps_lat: 6.6885, target_gps_lng: -1.6244, created_at: '2026-03-05T07:00:00Z', updated_at: '2026-03-05T08:30:00Z' },
  { id: 'job-004', job_reference: 'FJ-2026-0409', district_id: 'dist-001', district_name: 'Accra Metropolitan', account_number: 'GWL-ACC-012334', customer_name: 'Osu Castle Apartments', address: '5 Castle Road, Osu', status: 'COMPLETED', alert_level: 'MEDIUM', assigned_officer_id: 'officer-001', officer_name: 'Abena Owusu', target_gps_lat: 5.5560, target_gps_lng: -0.1969, created_at: '2026-03-04T09:00:00Z', updated_at: '2026-03-04T14:00:00Z' },
  { id: 'job-005', job_reference: 'FJ-2026-0408', district_id: 'dist-003', district_name: 'Tema Municipal', account_number: 'GWL-TEM-004412', customer_name: 'Meridian Shipping Co.', address: '22 Harbour Road, Tema', status: 'COMPLETED', alert_level: 'HIGH', assigned_officer_id: 'officer-003', officer_name: 'Yaa Asantewaa', target_gps_lat: 5.6698, target_gps_lng: 0.0166, created_at: '2026-03-03T09:00:00Z', updated_at: '2026-03-03T14:00:00Z' },
]

export const MOCK_NRW_SUMMARY = [
  { district_id: 'dist-001', district_name: 'Accra Metropolitan', region: 'Greater Accra', zone_type: 'URBAN', system_input_m3: 1820000, billed_m3: 944000, nrw_m3: 876000, nrw_pct: 48.1, apparent_loss_m3: 520000, real_loss_m3: 356000, loss_ratio_pct: 48.1, data_confidence_grade: 92, is_pilot_district: true, grade: 'D' },
  { district_id: 'dist-002', district_name: 'Kumasi Metropolitan', region: 'Ashanti', zone_type: 'URBAN', system_input_m3: 1540000, billed_m3: 738000, nrw_m3: 802000, nrw_pct: 52.1, apparent_loss_m3: 480000, real_loss_m3: 322000, loss_ratio_pct: 52.1, data_confidence_grade: 88, is_pilot_district: true, grade: 'F' },
  { district_id: 'dist-003', district_name: 'Tema Municipal', region: 'Greater Accra', zone_type: 'PERI_URBAN', system_input_m3: 680000, billed_m3: 376000, nrw_m3: 304000, nrw_pct: 44.7, apparent_loss_m3: 180000, real_loss_m3: 124000, loss_ratio_pct: 44.7, data_confidence_grade: 85, is_pilot_district: false, grade: 'D' },
  { district_id: 'dist-004', district_name: 'Tamale Metropolitan', region: 'Northern', zone_type: 'URBAN', system_input_m3: 520000, billed_m3: 201000, nrw_m3: 319000, nrw_pct: 61.3, apparent_loss_m3: 190000, real_loss_m3: 129000, loss_ratio_pct: 61.3, data_confidence_grade: 71, is_pilot_district: false, grade: 'F' },
  { district_id: 'dist-005', district_name: 'Cape Coast Municipal', region: 'Central', zone_type: 'PERI_URBAN', system_input_m3: 260000, billed_m3: 115000, nrw_m3: 145000, nrw_pct: 55.8, apparent_loss_m3: 88000, real_loss_m3: 57000, loss_ratio_pct: 55.8, data_confidence_grade: 79, is_pilot_district: false, grade: 'F' },
]

export const MOCK_REVENUE_SUMMARY = {
  total_events: 1274,
  total_variance_ghs: 8420000.00,
  total_recovered_ghs: 1842650.00,
  total_success_fee_ghs: 55279.50,
  pending_count: 47,
  confirmed_count: 312,
  collected_count: 891,
  by_type: [
    { recovery_type: 'SHADOW_BILL_VARIANCE', count: 612, recovered_ghs: 980000, success_fee_ghs: 29400 },
    { recovery_type: 'UNAUTHORISED_CONSUMPTION', count: 284, recovered_ghs: 520000, success_fee_ghs: 15600 },
    { recovery_type: 'PHANTOM_METER', count: 198, recovered_ghs: 220000, success_fee_ghs: 6600 },
    { recovery_type: 'GHOST_ACCOUNT', count: 180, recovered_ghs: 122650, success_fee_ghs: 3679.50 },
  ],
}

export const MOCK_WORKFORCE_SUMMARY = {
  total_field_officers: 24,
  active_now: 9,
  on_active_job: 7,
  idle_officers: 2,
  jobs_completed_today: 14,
}

export const MOCK_ACTIVE_OFFICERS = [
  { officer_id: 'officer-001', full_name: 'Abena Owusu', employee_id: 'EMP-0041', latitude: 5.6037, longitude: -0.1870, field_job_id: 'job-001', last_seen_at: '2026-03-06T12:14:00Z' },
  { officer_id: 'officer-002', full_name: 'Kweku Darko', employee_id: 'EMP-0038', latitude: 6.6885, longitude: -1.6244, field_job_id: 'job-003', last_seen_at: '2026-03-06T12:13:30Z' },
  { officer_id: 'officer-003', full_name: 'Yaa Asantewaa', employee_id: 'EMP-0029', latitude: 5.6698, longitude: 0.0166, field_job_id: null, last_seen_at: '2026-03-06T12:10:00Z' },
  { officer_id: 'officer-004', full_name: 'Ibrahim Mahama', employee_id: 'EMP-0052', latitude: 9.4008, longitude: -0.8393, field_job_id: 'job-002', last_seen_at: '2026-03-06T12:12:00Z' },
  { officer_id: 'officer-005', full_name: 'Nana Acheampong', employee_id: 'EMP-0047', latitude: 5.1053, longitude: -1.2466, field_job_id: null, last_seen_at: '2026-03-06T12:08:00Z' },
]

export const MOCK_USERS = [
  { id: 'admin-001', email: 'admin@gnwaas.gov.gh', full_name: 'Kwame Asante', role: 'SYSTEM_ADMIN', district_id: null, district_name: null, is_active: true, employee_id: 'EMP-0001', created_at: '2026-01-01T00:00:00Z' },
  { id: 'audit-001', email: 'audit.mgr@gnwaas.gov.gh', full_name: 'Efua Asare', role: 'AUDIT_MANAGER', district_id: null, district_name: null, is_active: true, employee_id: 'EMP-0002', created_at: '2026-01-01T00:00:00Z' },
  { id: 'gra-001', email: 'gra.auditor@gra.gov.gh', full_name: 'Kofi Boateng', role: 'GRA_AUDITOR', district_id: null, district_name: null, is_active: true, employee_id: 'EMP-0003', created_at: '2026-01-01T00:00:00Z' },
  { id: 'gwl-001', email: 'gwl.mgmt@gwl.com.gh', full_name: 'Ama Sarpong', role: 'GWL_MANAGEMENT', district_id: null, district_name: null, is_active: true, employee_id: 'EMP-0010', created_at: '2026-01-15T00:00:00Z' },
  { id: 'sup-001', email: 'supervisor@gnwaas.gov.gh', full_name: 'Yaw Mensah', role: 'FIELD_SUPERVISOR', district_id: 'dist-001', district_name: 'Accra Metropolitan', is_active: true, employee_id: 'EMP-0033', created_at: '2026-01-15T00:00:00Z' },
  { id: 'officer-001', email: 'abena.owusu@gnwaas.gov.gh', full_name: 'Abena Owusu', role: 'FIELD_OFFICER', district_id: 'dist-001', district_name: 'Accra Metropolitan', is_active: true, employee_id: 'EMP-0041', created_at: '2026-02-01T00:00:00Z' },
  { id: 'officer-002', email: 'kweku.darko@gnwaas.gov.gh', full_name: 'Kweku Darko', role: 'FIELD_OFFICER', district_id: 'dist-002', district_name: 'Kumasi Metropolitan', is_active: true, employee_id: 'EMP-0038', created_at: '2026-02-01T00:00:00Z' },
  { id: 'officer-003', email: 'yaa.asantewaa@gnwaas.gov.gh', full_name: 'Yaa Asantewaa', role: 'FIELD_OFFICER', district_id: 'dist-003', district_name: 'Tema Municipal', is_active: true, employee_id: 'EMP-0029', created_at: '2026-02-01T00:00:00Z' },
]

export const MOCK_SYSTEM_CONFIG = {
  audit: [
    { key: 'audit.variance_threshold_pct', value: '15', description: 'Minimum billing variance % to trigger an audit event', updated_at: '2026-03-01T00:00:00Z' },
    { key: 'audit.auto_escalate_days', value: '7', description: 'Days before open audit auto-escalates to manager', updated_at: '2026-03-01T00:00:00Z' },
    { key: 'audit.success_fee_pct', value: '3', description: 'Success fee percentage on recovered revenue', updated_at: '2026-03-01T00:00:00Z' },
  ],
  sentinel: [
    { key: 'sentinel.shadow_bill_variance_pct', value: '15', description: 'Shadow bill variance threshold (%)', updated_at: '2026-03-01T00:00:00Z' },
    { key: 'sentinel.night_flow_threshold_m3h', value: '2.5', description: 'Night flow threshold (m³/h) above which theft is suspected', updated_at: '2026-03-01T00:00:00Z' },
    { key: 'sentinel.ocr_confidence_threshold', value: '0.85', description: 'Minimum OCR confidence score to accept a meter reading', updated_at: '2026-03-01T00:00:00Z' },
  ],
  mobile: [
    { key: 'mobile.gps_accuracy_threshold_m', value: '50', description: 'Maximum GPS accuracy radius (metres) for field verification', updated_at: '2026-03-01T00:00:00Z' },
    { key: 'mobile.photo_required', value: 'true', description: 'Require meter photo for field job completion', updated_at: '2026-03-01T00:00:00Z' },
    { key: 'mobile.offline_sync_interval_min', value: '15', description: 'Offline data sync interval (minutes)', updated_at: '2026-03-01T00:00:00Z' },
  ],
  FIELD: [
    { key: 'mobile.gps_accuracy_threshold_m', value: '50', description: 'Maximum GPS accuracy radius (metres)', updated_at: '2026-03-01T00:00:00Z' },
    { key: 'mobile.photo_required', value: 'true', description: 'Require meter photo', updated_at: '2026-03-01T00:00:00Z' },
  ],
}

export const MOCK_REVENUE_EVENTS = [
  { id: 'rev-001', audit_reference: 'AUD-2026-1245', account_number: 'GWL-ACC-009981', customer_name: 'Accra Mall Management', district_name: 'Accra Metropolitan', recovery_type: 'SHADOW_BILL_VARIANCE', status: 'COLLECTED', recovered_ghs: 56400.00, success_fee_ghs: 1692.00, collected_at: '2026-03-04T16:22:00Z' },
  { id: 'rev-002', audit_reference: 'AUD-2026-1244', account_number: 'GWL-TEM-004412', customer_name: 'Meridian Shipping Co.', district_name: 'Tema Municipal', recovery_type: 'PHANTOM_METER', status: 'COLLECTED', recovered_ghs: 32800.00, success_fee_ghs: 984.00, collected_at: '2026-03-03T14:10:00Z' },
  { id: 'rev-003', audit_reference: 'AUD-2026-1246', account_number: 'GWL-KSI-002341', customer_name: 'Ama Boateng Cold Store', district_name: 'Kumasi Metropolitan', recovery_type: 'UNAUTHORISED_CONSUMPTION', status: 'CONFIRMED', recovered_ghs: 22100.00, success_fee_ghs: 663.00, collected_at: null },
]

// ─── Handlers ────────────────────────────────────────────────────────────────

export const handlers = [

  // ── Auth ──────────────────────────────────────────────────────────────────
  http.post(`${BASE}/auth/dev-login`, async () => {
    await d()
    return HttpResponse.json({ data: { access_token: MOCK_TOKEN, user: MOCK_USER } })
  }),
  http.post(`${BASE}/auth/login`, async () => {
    await d()
    return HttpResponse.json({ data: { access_token: MOCK_TOKEN, user: MOCK_USER } })
  }),
  http.post(`${BASE}/auth/refresh`, async () => {
    await d()
    return HttpResponse.json({ data: { access_token: MOCK_TOKEN } })
  }),

  // ── Current user ──────────────────────────────────────────────────────────
  http.get(`${BASE}/users/me`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_USER })
  }),

  // ── Districts ─────────────────────────────────────────────────────────────
  http.get(`${BASE}/districts`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_DISTRICTS })
  }),
  http.get(`${BASE}/districts/:id`, async ({ params }) => {
    await d()
    const dist = MOCK_DISTRICTS.find(d => d.id === params.id) || MOCK_DISTRICTS[0]
    return HttpResponse.json({ data: dist })
  }),
  http.post(`${BASE}/admin/districts`, async () => {
    await d()
    return HttpResponse.json({ data: { ...MOCK_DISTRICTS[0], id: 'dist-new-' + Date.now() } }, { status: 201 })
  }),
  http.patch(`${BASE}/admin/districts/:id`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_DISTRICTS[0] })
  }),

  // ── Dashboard stats ───────────────────────────────────────────────────────
  http.get(`${BASE}/audits/dashboard`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_DASHBOARD_STATS })
  }),

  // ── Anomaly flags  (admin uses /sentinel/anomalies) ───────────────────────
  http.get(`${BASE}/sentinel/anomalies`, async ({ request }) => {
    await d()
    const url = new URL(request.url)
    const status = url.searchParams.get('status')
    const severity = url.searchParams.get('severity') || url.searchParams.get('alert_level')
    const anomalyType = url.searchParams.get('anomaly_type')
    const districtId = url.searchParams.get('district_id')
    const limit = parseInt(url.searchParams.get('limit') || '50')
    const offset = parseInt(url.searchParams.get('offset') || '0')

    let flags = [...MOCK_ANOMALY_FLAGS]
    if (status) flags = flags.filter(f => f.status === status)
    if (severity) flags = flags.filter(f => f.alert_level === severity)
    if (anomalyType) flags = flags.filter(f => f.anomaly_type === anomalyType)
    if (districtId) flags = flags.filter(f => f.district_id === districtId)

    return HttpResponse.json({
      data: flags.slice(offset, offset + limit),
      meta: { total: flags.length, limit, offset },
    })
  }),
  http.get(`${BASE}/sentinel/anomalies/:id`, async ({ params }) => {
    await d()
    const flag = MOCK_ANOMALY_FLAGS.find(f => f.id === params.id) || MOCK_ANOMALY_FLAGS[0]
    return HttpResponse.json({ data: flag })
  }),
  http.post(`${BASE}/sentinel/anomalies`, async () => {
    await d()
    return HttpResponse.json({ data: { id: 'flag-new-' + Date.now(), flag_reference: 'ANF-2026-0900' } }, { status: 201 })
  }),

  // ── Sentinel summary & scan ───────────────────────────────────────────────
  http.get(`${BASE}/sentinel/summary/:districtId`, async () => {
    await d()
    return HttpResponse.json({ data: { total_anomalies: 8, critical: 1, high: 3, medium: 3, low: 1, last_run: '2026-03-06T05:00:00Z' } })
  }),
  http.post(`${BASE}/sentinel/scan/:districtId`, async () => {
    await d(800)
    return HttpResponse.json({ data: { triggered: true, job_id: 'scan-' + Date.now() } })
  }),

  // ── Audit events ──────────────────────────────────────────────────────────
  http.get(`${BASE}/audits`, async ({ request }) => {
    await d()
    const url = new URL(request.url)
    const status = url.searchParams.get('status')
    const districtId = url.searchParams.get('district_id')
    const graStatus = url.searchParams.get('gra_status')
    const limit = parseInt(url.searchParams.get('limit') || '50')
    const offset = parseInt(url.searchParams.get('offset') || '0')

    let events = [...MOCK_AUDIT_EVENTS]
    if (status) events = events.filter(e => e.status === status)
    if (districtId) events = events.filter(e => e.district_id === districtId)
    if (graStatus) events = events.filter(e => e.gra_status === graStatus)

    return HttpResponse.json({
      data: events.slice(offset, offset + limit),
      meta: { total: events.length, limit, offset },
    })
  }),
  http.get(`${BASE}/audits/:id`, async ({ params }) => {
    await d()
    const event = MOCK_AUDIT_EVENTS.find(e => e.id === params.id) || MOCK_AUDIT_EVENTS[0]
    return HttpResponse.json({ data: event })
  }),

  // ── Field jobs ────────────────────────────────────────────────────────────
  http.get(`${BASE}/field-jobs`, async ({ request }) => {
    await d()
    const url = new URL(request.url)
    const status = url.searchParams.get('status')
    const districtId = url.searchParams.get('district_id')
    const limit = parseInt(url.searchParams.get('limit') || '50')
    const offset = parseInt(url.searchParams.get('offset') || '0')

    let jobs = [...MOCK_FIELD_JOBS]
    if (status) jobs = jobs.filter(j => j.status === status)
    if (districtId) jobs = jobs.filter(j => j.district_id === districtId)

    return HttpResponse.json({
      data: jobs.slice(offset, offset + limit),
      meta: { total: jobs.length, limit, offset },
    })
  }),
  http.patch(`${BASE}/field-jobs/:id/assign`, async () => {
    await d()
    return HttpResponse.json({ data: { ...MOCK_FIELD_JOBS[1], status: 'ASSIGNED', assigned_officer_id: 'officer-001', officer_name: 'Abena Owusu' } })
  }),

  // ── NRW reports ───────────────────────────────────────────────────────────
  http.get(`${BASE}/reports/nrw`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_NRW_SUMMARY })
  }),

  // ── Water balance ─────────────────────────────────────────────────────────
  http.get(`${BASE}/water-balance`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_WATER_BALANCE })
  }),

  // ── Revenue ───────────────────────────────────────────────────────────────
  http.get(`${BASE}/revenue/summary`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_REVENUE_SUMMARY })
  }),
  http.get(`${BASE}/revenue/events`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_REVENUE_EVENTS, meta: { total: MOCK_REVENUE_EVENTS.length } })
  }),

  // ── Workforce ─────────────────────────────────────────────────────────────
  http.get(`${BASE}/workforce/summary`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_WORKFORCE_SUMMARY })
  }),
  http.get(`${BASE}/workforce/active`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_ACTIVE_OFFICERS })
  }),

  // ── GRA compliance ────────────────────────────────────────────────────────
  http.post(`${BASE}/gra/submit/:id`, async () => {
    await d(600)
    return HttpResponse.json({ data: { submitted: true, receipt_number: 'GRA-2026-' + Date.now() } })
  }),

  // ── Reports (download) ────────────────────────────────────────────────────
  http.get(`${BASE}/reports/monthly/pdf`, async () => {
    await d(400)
    return new HttpResponse('Mock PDF content', { headers: { 'Content-Type': 'application/pdf', 'Content-Disposition': 'attachment; filename="nrw-report-2026-03.pdf"' } })
  }),
  http.get(`${BASE}/reports/monthly/csv`, async () => {
    await d(300)
    return new HttpResponse('district,nrw_pct,system_input_m3\nAccra Metropolitan,48.1,1820000\nKumasi Metropolitan,52.1,1540000', { headers: { 'Content-Type': 'text/csv', 'Content-Disposition': 'attachment; filename="nrw-report-2026-03.csv"' } })
  }),
  http.get(`${BASE}/reports/gra-compliance/csv`, async () => {
    await d(300)
    return new HttpResponse('audit_ref,account,gra_status\nAUD-2026-1245,GWL-ACC-009981,CONFIRMED', { headers: { 'Content-Type': 'text/csv' } })
  }),
  http.get(`${BASE}/reports/audit-trail/csv`, async () => {
    await d(300)
    return new HttpResponse('audit_ref,status,variance_pct\nAUD-2026-1247,IN_PROGRESS,67.4', { headers: { 'Content-Type': 'text/csv' } })
  }),
  http.get(`${BASE}/reports/field-jobs/csv`, async () => {
    await d(300)
    return new HttpResponse('job_ref,status,officer\nFJ-2026-0412,ON_SITE,Abena Owusu', { headers: { 'Content-Type': 'text/csv' } })
  }),

  // ── System config ─────────────────────────────────────────────────────────
  http.get(`${BASE}/config/:category`, async ({ params }) => {
    await d()
    const cat = params.category as string
    const items = MOCK_SYSTEM_CONFIG[cat as keyof typeof MOCK_SYSTEM_CONFIG]
      || MOCK_SYSTEM_CONFIG.audit
    return HttpResponse.json({ data: items })
  }),
  http.patch(`${BASE}/config/:key`, async ({ params }) => {
    await d()
    return HttpResponse.json({ data: { key: params.key, value: 'updated', updated_at: new Date().toISOString() } })
  }),

  // ── Admin users ───────────────────────────────────────────────────────────
  http.get(`${BASE}/admin/users`, async ({ request }) => {
    await d()
    const url = new URL(request.url)
    const role = url.searchParams.get('role')
    const search = url.searchParams.get('search') || ''
    const limit = parseInt(url.searchParams.get('limit') || '50')
    const offset = parseInt(url.searchParams.get('offset') || '0')

    let users = [...MOCK_USERS]
    if (role) users = users.filter(u => u.role === role)
    if (search) users = users.filter(u => u.full_name.toLowerCase().includes(search.toLowerCase()) || u.email.includes(search))

    return HttpResponse.json({ data: users.slice(offset, offset + limit), meta: { total: users.length } })
  }),
  http.post(`${BASE}/admin/users`, async () => {
    await d()
    return HttpResponse.json({ data: { ...MOCK_USERS[0], id: 'user-new-' + Date.now() } }, { status: 201 })
  }),
  http.patch(`${BASE}/admin/users/:id`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_USERS[0] })
  }),
  http.patch(`${BASE}/admin/users/:id/status`, async () => {
    await d()
    return HttpResponse.json({ data: { ...MOCK_USERS[0], is_active: false } })
  }),
  http.post(`${BASE}/admin/users/:id/reset-password`, async () => {
    await d()
    return HttpResponse.json({ data: { reset: true } })
  }),
  http.get(`${BASE}/users/field-officers`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_USERS.filter(u => u.role === 'FIELD_OFFICER') })
  }),
]
