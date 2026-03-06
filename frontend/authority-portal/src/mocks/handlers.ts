import { http, HttpResponse, delay } from 'msw'

const BASE = '/api/v1'
const d = (ms = 200) => delay(ms)

const MOCK_AUTH_TOKEN = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhdXRoLXVzZXItMDAxIiwicm9sZSI6IkZJRUxEX1NVUEVSVklTT1IiLCJleHAiOjk5OTk5OTk5OTl9.mock'

const MOCK_USER = { id: 'auth-user-001', email: 'supervisor@gnwaas.gov.gh', full_name: 'Efua Asare', role: 'FIELD_SUPERVISOR', district_id: 'dist-001', is_active: true, employee_id: 'EMP-0033' }

const MOCK_DISTRICTS = [
  { id: 'dist-001', district_name: 'Accra Metropolitan', region: 'Greater Accra', zone_type: 'URBAN', gps_latitude: 5.6037, gps_longitude: -0.1870, is_pilot_district: true, loss_ratio_pct: 48.2 },
  { id: 'dist-002', district_name: 'Kumasi Metropolitan', region: 'Ashanti', zone_type: 'URBAN', gps_latitude: 6.6885, gps_longitude: -1.6244, is_pilot_district: true, loss_ratio_pct: 52.1 },
  { id: 'dist-003', district_name: 'Tema Municipal', region: 'Greater Accra', zone_type: 'PERI_URBAN', gps_latitude: 5.6698, gps_longitude: 0.0166, is_pilot_district: false, loss_ratio_pct: 44.7 },
  { id: 'dist-004', district_name: 'Tamale Metropolitan', region: 'Northern', zone_type: 'URBAN', gps_latitude: 9.4008, gps_longitude: -0.8393, is_pilot_district: false, loss_ratio_pct: 61.3 },
  { id: 'dist-005', district_name: 'Cape Coast Municipal', region: 'Central', zone_type: 'PERI_URBAN', gps_latitude: 5.1053, gps_longitude: -1.2466, is_pilot_district: false, loss_ratio_pct: 55.8 },
]

const MOCK_ACCOUNTS = [
  { id: 'acc-001', account_number: 'GWL-ACC-004821', customer_name: 'Kofi Mensah Enterprises', address: '14 Ring Road East, Accra', district_id: 'dist-001', district_name: 'Accra Metropolitan', meter_serial: 'MTR-ACC-004821', account_type: 'COMMERCIAL', is_active: true, monthly_consumption_m3: 420.5 },
  { id: 'acc-002', account_number: 'GWL-KSI-002341', customer_name: 'Ama Boateng Cold Store', address: '7 Adum Road, Kumasi', district_id: 'dist-002', district_name: 'Kumasi Metropolitan', meter_serial: 'MTR-KSI-002341', account_type: 'COMMERCIAL', is_active: true, monthly_consumption_m3: 310.2 },
  { id: 'acc-003', account_number: 'GWL-ACC-007712', customer_name: 'Tema Industrial Park Ltd', address: '22 Industrial Ave, Tema', district_id: 'dist-003', district_name: 'Tema Municipal', meter_serial: 'MTR-TEM-007712', account_type: 'INDUSTRIAL', is_active: true, monthly_consumption_m3: 1840.0 },
  { id: 'acc-004', account_number: 'GWL-TEM-001198', customer_name: 'Harbour View Hotel', address: '22 Harbour Road, Tema', district_id: 'dist-003', district_name: 'Tema Municipal', meter_serial: 'MTR-TEM-001198', account_type: 'COMMERCIAL', is_active: true, monthly_consumption_m3: 280.8 },
  { id: 'acc-005', account_number: 'GWL-ACC-012334', customer_name: 'Osu Castle Apartments', address: '5 Castle Road, Osu, Accra', district_id: 'dist-001', district_name: 'Accra Metropolitan', meter_serial: 'MTR-ACC-012334', account_type: 'RESIDENTIAL', is_active: true, monthly_consumption_m3: 42.3 },
]

const MOCK_NRW_SUMMARY = [
  { district_id: 'dist-001', district_name: 'Accra Metropolitan', region: 'Greater Accra', zone_type: 'URBAN', system_input_m3: 1820000, billed_m3: 944000, nrw_m3: 876000, nrw_pct: 48.1, apparent_loss_m3: 520000, real_loss_m3: 356000, loss_ratio_pct: 48.1, data_confidence_grade: 92, is_pilot_district: true, grade: 'D' },
  { district_id: 'dist-002', district_name: 'Kumasi Metropolitan', region: 'Ashanti', zone_type: 'URBAN', system_input_m3: 1540000, billed_m3: 738000, nrw_m3: 802000, nrw_pct: 52.1, apparent_loss_m3: 480000, real_loss_m3: 322000, loss_ratio_pct: 52.1, data_confidence_grade: 88, is_pilot_district: true, grade: 'F' },
  { district_id: 'dist-003', district_name: 'Tema Municipal', region: 'Greater Accra', zone_type: 'PERI_URBAN', system_input_m3: 680000, billed_m3: 376000, nrw_m3: 304000, nrw_pct: 44.7, apparent_loss_m3: 180000, real_loss_m3: 124000, loss_ratio_pct: 44.7, data_confidence_grade: 85, is_pilot_district: false, grade: 'D' },
  { district_id: 'dist-004', district_name: 'Tamale Metropolitan', region: 'Northern', zone_type: 'URBAN', system_input_m3: 520000, billed_m3: 201000, nrw_m3: 319000, nrw_pct: 61.3, apparent_loss_m3: 190000, real_loss_m3: 129000, loss_ratio_pct: 61.3, data_confidence_grade: 71, is_pilot_district: false, grade: 'F' },
  { district_id: 'dist-005', district_name: 'Cape Coast Municipal', region: 'Central', zone_type: 'PERI_URBAN', system_input_m3: 260000, billed_m3: 115000, nrw_m3: 145000, nrw_pct: 55.8, apparent_loss_m3: 88000, real_loss_m3: 57000, loss_ratio_pct: 55.8, data_confidence_grade: 79, is_pilot_district: false, grade: 'F' },
]

const MOCK_FIELD_JOBS = [
  { id: 'job-001', job_reference: 'FJ-2026-0412', district_id: 'dist-001', district_name: 'Accra Metropolitan', account_number: 'GWL-ACC-004821', customer_name: 'Kofi Mensah Enterprises', address: '14 Ring Road East, Accra', status: 'ON_SITE', alert_level: 'CRITICAL', assigned_officer_id: 'auth-user-001', officer_name: 'Efua Asare', target_gps_lat: 5.6037, target_gps_lng: -0.1870, created_at: '2026-03-05T09:00:00Z', updated_at: '2026-03-05T10:45:00Z' },
  { id: 'job-002', job_reference: 'FJ-2026-0411', district_id: 'dist-001', district_name: 'Accra Metropolitan', account_number: 'GWL-ACC-007712', customer_name: 'Tema Industrial Park Ltd', address: '22 Industrial Ave, Tema', status: 'QUEUED', alert_level: 'HIGH', assigned_officer_id: null, officer_name: null, target_gps_lat: 5.6698, target_gps_lng: 0.0166, created_at: '2026-03-05T08:00:00Z', updated_at: '2026-03-05T08:00:00Z' },
  { id: 'job-003', job_reference: 'FJ-2026-0410', district_id: 'dist-001', district_name: 'Accra Metropolitan', account_number: 'GWL-ACC-012334', customer_name: 'Osu Castle Apartments', address: '5 Castle Road, Osu', status: 'COMPLETED', alert_level: 'MEDIUM', assigned_officer_id: 'auth-user-001', officer_name: 'Efua Asare', target_gps_lat: 5.5560, target_gps_lng: -0.1969, created_at: '2026-03-04T09:00:00Z', updated_at: '2026-03-04T14:00:00Z' },
]

const MOCK_ANOMALY_FLAGS = [
  { id: 'flag-001', flag_reference: 'ANF-2026-0847', district_id: 'dist-001', account_number: 'GWL-ACC-004821', customer_name: 'Kofi Mensah Enterprises', anomaly_type: 'SHADOW_BILL_VARIANCE', alert_level: 'CRITICAL', status: 'OPEN', estimated_loss_ghs: 48200.00, variance_pct: 67.4, created_at: '2026-03-05T08:14:22Z' },
  { id: 'flag-002', flag_reference: 'ANF-2026-0846', district_id: 'dist-001', account_number: 'GWL-ACC-007712', customer_name: 'Tema Industrial Park Ltd', anomaly_type: 'NIGHT_FLOW_ANOMALY', alert_level: 'HIGH', status: 'ACKNOWLEDGED', estimated_loss_ghs: 18750.00, variance_pct: 38.9, created_at: '2026-03-04T22:15:44Z' },
  { id: 'flag-003', flag_reference: 'ANF-2026-0845', district_id: 'dist-001', account_number: 'GWL-ACC-012334', customer_name: 'Osu Castle Apartments', anomaly_type: 'METERING_INACCURACY', alert_level: 'MEDIUM', status: 'RESOLVED', estimated_loss_ghs: 6100.00, variance_pct: 16.8, created_at: '2026-03-03T08:55:30Z' },
]

const MOCK_FIELD_OFFICERS = [
  { id: 'officer-001', full_name: 'Abena Owusu', employee_id: 'EMP-0041', role: 'FIELD_OFFICER', district_name: 'Accra Metropolitan', status: 'ON_FIELD', is_mfa_enabled: true },
  { id: 'officer-002', full_name: 'Kweku Darko', employee_id: 'EMP-0038', role: 'FIELD_OFFICER', district_name: 'Accra Metropolitan', status: 'AVAILABLE', is_mfa_enabled: true },
  { id: 'officer-003', full_name: 'Yaa Asantewaa', employee_id: 'EMP-0029', role: 'FIELD_OFFICER', district_name: 'Accra Metropolitan', status: 'AVAILABLE', is_mfa_enabled: false },
  { id: 'officer-004', full_name: 'Nana Acheampong', employee_id: 'EMP-0047', role: 'FIELD_OFFICER', district_name: 'Accra Metropolitan', status: 'ON_FIELD', is_mfa_enabled: true },
]

const MOCK_AUDIT_EVENTS = [
  { id: 'audit-001', audit_reference: 'AUD-2026-1247', account_number: 'GWL-ACC-004821', account_holder: 'Kofi Mensah Enterprises', district_name: 'Accra Metropolitan', status: 'IN_PROGRESS', gra_status: 'PENDING', gwl_billed_ghs: 71400.00, shadow_bill_ghs: 119600.00, variance_pct: 67.4, created_at: '2026-03-05T08:14:22Z' },
  { id: 'audit-002', audit_reference: 'AUD-2026-1246', account_number: 'GWL-ACC-007712', account_holder: 'Tema Industrial Park Ltd', district_name: 'Accra Metropolitan', status: 'AWAITING_GRA', gra_status: 'PENDING', gwl_billed_ghs: 53700.00, shadow_bill_ghs: 75800.00, variance_pct: 41.2, created_at: '2026-03-05T07:30:11Z' },
]

export const handlers = [
  // ── Auth ──────────────────────────────────────────────────
  http.post(`${BASE}/auth/dev-login`, async () => {
    await d()
    return HttpResponse.json({ data: { access_token: MOCK_AUTH_TOKEN, user: MOCK_USER } })
  }),
  http.post(`${BASE}/auth/login`, async () => {
    await d()
    return HttpResponse.json({ data: { access_token: MOCK_AUTH_TOKEN, user: MOCK_USER } })
  }),
  http.post(`${BASE}/auth/refresh`, async () => {
    await d()
    return HttpResponse.json({ data: { access_token: MOCK_AUTH_TOKEN } })
  }),

  // ── Current user ──────────────────────────────────────────
  http.get(`${BASE}/users/me`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_USER })
  }),

  // ── Districts ─────────────────────────────────────────────
  http.get(`${BASE}/districts`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_DISTRICTS })
  }),
  http.get(`${BASE}/districts/:id`, async ({ params }) => {
    await d()
    const dist = MOCK_DISTRICTS.find(d => d.id === params.id) || MOCK_DISTRICTS[0]
    return HttpResponse.json({ data: dist })
  }),

  // ── Accounts ──────────────────────────────────────────────
  http.get(`${BASE}/accounts`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_ACCOUNTS, meta: { total: MOCK_ACCOUNTS.length, page: 1, page_size: 20 } })
  }),
  http.get(`${BASE}/accounts/search`, async ({ request }) => {
    await d()
    const url = new URL(request.url)
    const q = url.searchParams.get('q') || ''
    const results = MOCK_ACCOUNTS.filter(a => a.account_number.includes(q) || a.customer_name.toLowerCase().includes(q.toLowerCase()))
    return HttpResponse.json({ data: results, meta: { total: results.length, page: 1, page_size: 20 } })
  }),
  http.get(`${BASE}/accounts/:id`, async ({ params }) => {
    await d()
    const acc = MOCK_ACCOUNTS.find(a => a.id === params.id) || MOCK_ACCOUNTS[0]
    return HttpResponse.json({ data: acc })
  }),

  // ── NRW Reports ───────────────────────────────────────────
  http.get(`${BASE}/reports/nrw`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_NRW_SUMMARY })
  }),
  http.get(`${BASE}/reports/nrw/my-district`, async () => {
    await d()
    return HttpResponse.json({ data: { district: MOCK_DISTRICTS[0], summary: MOCK_NRW_SUMMARY[0], trend: [] } })
  }),
  http.get(`${BASE}/reports/nrw/trend`, async () => {
    await d()
    return HttpResponse.json({ data: [
      { month: '2026-01', nrw_pct: 53.2, system_input_m3: 1780000 },
      { month: '2026-02', nrw_pct: 51.8, system_input_m3: 1800000 },
      { month: '2026-03', nrw_pct: 48.1, system_input_m3: 1820000 },
    ]})
  }),

  // ── Audit events ──────────────────────────────────────────
  http.get(`${BASE}/audits/dashboard`, async () => {
    await d()
    return HttpResponse.json({ data: { pending: 47, in_progress: 23, total_confirmed_loss_ghs: 1842650, gra_signed: 312, total_success_fees_ghs: 55279.50, completed: 891, total: 1274, pending_assignment: 31 } })
  }),
  http.get(`${BASE}/audits`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_AUDIT_EVENTS, meta: { total: MOCK_AUDIT_EVENTS.length } })
  }),
  http.get(`${BASE}/audits/:id`, async ({ params }) => {
    await d()
    const e = MOCK_AUDIT_EVENTS.find(a => a.id === params.id) || MOCK_AUDIT_EVENTS[0]
    return HttpResponse.json({ data: e })
  }),
  http.post(`${BASE}/audits`, async () => {
    await d()
    return HttpResponse.json({ data: { id: 'new-audit-' + Date.now(), audit_reference: 'AUD-2026-1300' } }, { status: 201 })
  }),

  // ── Field jobs ────────────────────────────────────────────
  http.get(`${BASE}/field-jobs/my-jobs`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_FIELD_JOBS.filter(j => j.assigned_officer_id === 'auth-user-001') })
  }),
  http.get(`${BASE}/field-jobs`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_FIELD_JOBS })
  }),
  http.patch(`${BASE}/field-jobs/:id/status`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_FIELD_JOBS[0] })
  }),
  http.post(`${BASE}/field-jobs/:id/sos`, async () => {
    await d()
    return HttpResponse.json({ data: { triggered: true } })
  }),
  http.patch(`${BASE}/field-jobs/:id/assign`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_FIELD_JOBS[0] })
  }),

  // ── Anomaly flags ─────────────────────────────────────────
  http.get(`${BASE}/anomaly-flags`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_ANOMALY_FLAGS })
  }),
  http.post(`${BASE}/sentinel/anomalies`, async () => {
    await d()
    return HttpResponse.json({ data: { id: 'new-flag-' + Date.now(), flag_reference: 'ANF-2026-0900' } }, { status: 201 })
  }),

  // ── Field officers ────────────────────────────────────────
  http.get(`${BASE}/users/field-officers`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_FIELD_OFFICERS })
  }),

  // ── Sentinel ──────────────────────────────────────────────
  http.get(`${BASE}/sentinel/summary/:districtId`, async () => {
    await d()
    return HttpResponse.json({ data: { total_anomalies: 12, critical: 1, high: 3, medium: 6, low: 2, last_run: '2026-03-06T05:00:00Z' } })
  }),

  // ── Water balance ─────────────────────────────────────────
  http.get(`${BASE}/water-balance`, async () => {
    await d()
    return HttpResponse.json({ data: { system_input_m3: 1820000, nrw_m3: 876000, nrw_pct: 48.1, apparent_losses_m3: 520000, real_losses_m3: 356000, period_start: '2026-03-01T00:00:00Z', period_end: '2026-03-05T23:59:59Z' } })
  }),

  // ── Reports ───────────────────────────────────────────────
  http.get(`${BASE}/reports/monthly/:format`, async ({ params }) => {
    await d(400)
    if (params.format === 'pdf') {
      return new HttpResponse('Mock PDF', { headers: { 'Content-Type': 'application/pdf' } })
    }
    return new HttpResponse('district,nrw_pct\nAccra Metropolitan,48.1', { headers: { 'Content-Type': 'text/csv' } })
  }),
]
