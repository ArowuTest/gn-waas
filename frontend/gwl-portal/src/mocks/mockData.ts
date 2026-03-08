// ============================================================
// GN-WAAS GWL Portal — Mock Data
// ============================================================

export const MOCK_GWL_TOKEN = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJnd2wtdXNlci0wMDEiLCJyb2xlIjoiR1dMX01BTkFHRU1FTlQiLCJleHAiOjk5OTk5OTk5OTl9.mock'

export const MOCK_GWL_USER = {
  id: 'gwl-user-001',
  email: 'gwl.mgmt@gwl.com.gh',
  full_name: 'Ama Sarpong',
  role: 'GWL_MANAGEMENT',
  district_id: null,
  is_active: true,
}

export const MOCK_CASE_SUMMARY = {
  total_cases: 1274,
  open_cases: 312,
  in_progress: 89,
  pending_field: 47,
  resolved: 826,
  total_variance_ghs: 8420000.00,
  total_confirmed_loss_ghs: 1842650.00,
  avg_resolution_days: 4.2,
}

export const MOCK_CASES = [
  { id: 'case-001', audit_reference: 'AUD-2026-1247', account_number: 'GWL-ACC-004821', customer_name: 'Kofi Mensah Enterprises', district_name: 'Accra Metropolitan', anomaly_type: 'SHADOW_BILL_VARIANCE', alert_level: 'CRITICAL', gwl_status: 'OPEN', gwl_billed_ghs: 71400.00, shadow_bill_ghs: 119600.00, variance_pct: 67.4, assigned_officer: null, created_at: '2026-03-05T08:14:22Z', updated_at: '2026-03-05T10:30:00Z' },
  { id: 'case-002', audit_reference: 'AUD-2026-1246', account_number: 'GWL-KSI-002341', customer_name: 'Ama Boateng Cold Store', district_name: 'Kumasi Metropolitan', anomaly_type: 'UNAUTHORISED_CONSUMPTION', alert_level: 'HIGH', gwl_status: 'FIELD_ASSIGNED', gwl_billed_ghs: 53700.00, shadow_bill_ghs: 75800.00, variance_pct: 41.2, assigned_officer: 'Kweku Darko', created_at: '2026-03-05T07:30:11Z', updated_at: '2026-03-05T09:15:00Z' },
  { id: 'case-003', audit_reference: 'AUD-2026-1245', account_number: 'GWL-ACC-009981', customer_name: 'Accra Mall Management', district_name: 'Accra Metropolitan', anomaly_type: 'SHADOW_BILL_VARIANCE', alert_level: 'HIGH', gwl_status: 'CORRECTED', gwl_billed_ghs: 142000.00, shadow_bill_ghs: 198400.00, variance_pct: 39.7, assigned_officer: 'Abena Owusu', created_at: '2026-03-03T11:00:00Z', updated_at: '2026-03-04T16:22:00Z' },
  { id: 'case-004', audit_reference: 'AUD-2026-1244', account_number: 'GWL-TEM-004412', customer_name: 'Meridian Shipping Co.', district_name: 'Tema Municipal', anomaly_type: 'PHANTOM_METER', alert_level: 'HIGH', gwl_status: 'CLOSED', gwl_billed_ghs: 88200.00, shadow_bill_ghs: 121000.00, variance_pct: 37.2, assigned_officer: 'Yaa Asantewaa', created_at: '2026-03-02T09:00:00Z', updated_at: '2026-03-03T14:10:00Z' },
  { id: 'case-005', audit_reference: 'AUD-2026-1243', account_number: 'GWL-KSI-007823', customer_name: 'Kumasi Central Market Assoc.', district_name: 'Kumasi Metropolitan', anomaly_type: 'CATEGORY_MISMATCH', alert_level: 'MEDIUM', gwl_status: 'OPEN', gwl_billed_ghs: 34100.00, shadow_bill_ghs: 46200.00, variance_pct: 35.5, assigned_officer: null, created_at: '2026-03-05T06:00:00Z', updated_at: '2026-03-05T06:00:00Z' },
  { id: 'case-006', audit_reference: 'AUD-2026-1242', account_number: 'GWL-TAM-000892', customer_name: 'Northern Star Filling Station', district_name: 'Tamale Metropolitan', anomaly_type: 'ADDRESS_UNVERIFIED', alert_level: 'MEDIUM', gwl_status: 'DISPUTED', gwl_billed_ghs: 28400.00, shadow_bill_ghs: 37600.00, variance_pct: 32.4, assigned_officer: 'Ibrahim Mahama', created_at: '2026-03-04T14:00:00Z', updated_at: '2026-03-05T08:00:00Z' },
]

export const MOCK_CASE_ACTIONS = [
  { id: 'action-001', case_id: 'case-001', action_type: 'STATUS_CHANGE', performed_by: 'Ama Sarpong', notes: 'Case opened from anomaly flag ANF-2026-0847', created_at: '2026-03-05T08:14:22Z' },
  { id: 'action-002', case_id: 'case-001', action_type: 'FIELD_ASSIGNED', performed_by: 'Efua Asare', notes: 'Assigned to field officer Abena Owusu for physical verification', created_at: '2026-03-05T09:00:00Z' },
]

export const MOCK_RECLASSIFICATIONS = [
  { id: 'recl-001', case_id: 'case-006', audit_reference: 'AUD-2026-1242', account_number: 'GWL-TAM-000892', customer_name: 'Northern Star Filling Station', current_category: 'COMMERCIAL', requested_category: 'INDUSTRIAL', reason: 'Customer operates heavy machinery — should be classified as industrial', status: 'PENDING', requested_by: 'Ama Sarpong', created_at: '2026-03-05T08:00:00Z' },
  { id: 'recl-002', case_id: 'case-005', audit_reference: 'AUD-2026-1243', account_number: 'GWL-KSI-007823', customer_name: 'Kumasi Central Market Assoc.', current_category: 'COMMERCIAL', requested_category: 'RESIDENTIAL', reason: 'Market association is non-profit community entity', status: 'APPROVED', requested_by: 'Ama Sarpong', created_at: '2026-03-04T10:00:00Z' },
]

export const MOCK_CREDITS = [
  { id: 'cred-001', case_id: 'case-003', audit_reference: 'AUD-2026-1245', account_number: 'GWL-ACC-009981', customer_name: 'Accra Mall Management', credit_amount_ghs: 12400.00, reason: 'Overbilling during meter fault period Jan-Feb 2026', status: 'APPROVED', requested_by: 'Ama Sarpong', created_at: '2026-03-04T12:00:00Z' },
  { id: 'cred-002', case_id: 'case-004', audit_reference: 'AUD-2026-1244', account_number: 'GWL-TEM-004412', customer_name: 'Meridian Shipping Co.', credit_amount_ghs: 8800.00, reason: 'Billing during supply outage period', status: 'PENDING', requested_by: 'Ama Sarpong', created_at: '2026-03-05T07:00:00Z' },
]

export const MOCK_FIELD_OFFICERS = [
  { id: 'officer-001', full_name: 'Abena Owusu', employee_id: 'EMP-0041', role: 'FIELD_OFFICER', district_name: 'Accra Metropolitan', status: 'ON_FIELD' },
  { id: 'officer-002', full_name: 'Kweku Darko', employee_id: 'EMP-0038', role: 'FIELD_OFFICER', district_name: 'Kumasi Metropolitan', status: 'ON_FIELD' },
  { id: 'officer-003', full_name: 'Yaa Asantewaa', employee_id: 'EMP-0029', role: 'FIELD_OFFICER', district_name: 'Tema Municipal', status: 'AVAILABLE' },
  { id: 'officer-004', full_name: 'Ibrahim Mahama', employee_id: 'EMP-0052', role: 'FIELD_OFFICER', district_name: 'Tamale Metropolitan', status: 'SOS' },
]

export const MOCK_MONTHLY_REPORT = {
  period: '2026-03',
  district_name: 'All Districts',
  total_accounts: 48420,
  total_billed_ghs: 12840000.00,
  total_shadow_ghs: 18920000.00,
  total_variance_ghs: 6080000.00,
  cases_opened: 312,
  cases_resolved: 189,
  recovery_rate_pct: 60.6,
  generated_at: '2026-03-06T00:00:00Z',
}
