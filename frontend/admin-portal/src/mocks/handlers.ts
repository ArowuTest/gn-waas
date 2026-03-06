// ============================================================
// GN-WAAS Admin Portal — MSW Mock Handlers
// ============================================================
import { http, HttpResponse, delay } from 'msw'
import {
  MOCK_TOKEN, MOCK_USER, MOCK_DISTRICTS, MOCK_DASHBOARD_STATS,
  MOCK_ANOMALY_FLAGS, MOCK_AUDIT_EVENTS, MOCK_FIELD_JOBS,
  MOCK_WATER_BALANCE, MOCK_REVENUE_SUMMARY, MOCK_WORKFORCE_SUMMARY,
  MOCK_ACTIVE_OFFICERS, MOCK_NRW_SUMMARY, MOCK_USERS, MOCK_SYSTEM_CONFIG,
} from './mockData'

const BASE = '/api/v1'
const d = (ms = 200) => delay(ms)

export const handlers = [
  // ── Auth ──────────────────────────────────────────────────
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

  // ── Current user ──────────────────────────────────────────
  http.get(`${BASE}/users/me`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_USER })
  }),

  // ── Users ─────────────────────────────────────────────────
  http.get(`${BASE}/users`, async ({ request }) => {
    await d()
    const url = new URL(request.url)
    const page = parseInt(url.searchParams.get('page') || '1')
    const pageSize = parseInt(url.searchParams.get('page_size') || '20')
    const search = url.searchParams.get('search') || ''
    let users = MOCK_USERS
    if (search) users = users.filter(u => u.full_name.toLowerCase().includes(search.toLowerCase()) || u.email.includes(search))
    const start = (page - 1) * pageSize
    return HttpResponse.json({ data: users.slice(start, start + pageSize), meta: { total: users.length, page, page_size: pageSize } })
  }),
  http.post(`${BASE}/users`, async () => {
    await d()
    return HttpResponse.json({ data: { ...MOCK_USERS[0], id: 'new-user-' + Date.now() } }, { status: 201 })
  }),
  http.patch(`${BASE}/users/:id`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_USERS[0] })
  }),
  http.delete(`${BASE}/users/:id`, async () => {
    await d()
    return HttpResponse.json({ message: 'User deactivated' })
  }),

  // ── Districts ─────────────────────────────────────────────
  http.get(`${BASE}/districts`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_DISTRICTS, meta: { total: MOCK_DISTRICTS.length } })
  }),
  http.post(`${BASE}/districts`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_DISTRICTS[0] }, { status: 201 })
  }),
  http.patch(`${BASE}/districts/:id`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_DISTRICTS[0] })
  }),

  // ── Dashboard stats ───────────────────────────────────────
  http.get(`${BASE}/audit-events/stats`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_DASHBOARD_STATS })
  }),

  // ── Anomaly flags ─────────────────────────────────────────
  http.get(`${BASE}/anomaly-flags`, async ({ request }) => {
    await d()
    const url = new URL(request.url)
    const severity = url.searchParams.get('severity') || ''
    const status = url.searchParams.get('status') || ''
    const anomalyType = url.searchParams.get('anomaly_type') || ''
    let flags = [...MOCK_ANOMALY_FLAGS]
    if (severity) flags = flags.filter(f => f.alert_level === severity)
    if (status) flags = flags.filter(f => f.status === status)
    if (anomalyType) flags = flags.filter(f => f.anomaly_type === anomalyType)
    const limit = parseInt(url.searchParams.get('limit') || '50')
    const offset = parseInt(url.searchParams.get('offset') || '0')
    return HttpResponse.json({ data: flags.slice(offset, offset + limit), meta: { total: flags.length } })
  }),
  http.patch(`${BASE}/anomaly-flags/:id/status`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_ANOMALY_FLAGS[0] })
  }),

  // ── Audit events ──────────────────────────────────────────
  http.get(`${BASE}/audits`, async ({ request }) => {
    await d()
    const url = new URL(request.url)
    const status = url.searchParams.get('status') || ''
    let events = [...MOCK_AUDIT_EVENTS]
    if (status) events = events.filter(e => e.status === status)
    const limit = parseInt(url.searchParams.get('limit') || '50')
    const offset = parseInt(url.searchParams.get('offset') || '0')
    return HttpResponse.json({ data: events.slice(offset, offset + limit), meta: { total: events.length } })
  }),
  http.get(`${BASE}/audits/:id`, async ({ params }) => {
    await d()
    const event = MOCK_AUDIT_EVENTS.find(e => e.id === params.id) || MOCK_AUDIT_EVENTS[0]
    return HttpResponse.json({ data: event })
  }),
  http.patch(`${BASE}/audits/:id/status`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_AUDIT_EVENTS[0] })
  }),
  http.post(`${BASE}/audits/:id/lock`, async () => {
    await d()
    return HttpResponse.json({ data: { ...MOCK_AUDIT_EVENTS[2], is_locked: true } })
  }),

  // ── Field jobs ────────────────────────────────────────────
  http.get(`${BASE}/field-jobs`, async ({ request }) => {
    await d()
    const url = new URL(request.url)
    const status = url.searchParams.get('status') || ''
    let jobs = [...MOCK_FIELD_JOBS]
    if (status && status !== 'ALL') jobs = jobs.filter(j => j.status === status)
    const limit = parseInt(url.searchParams.get('limit') || '50')
    const offset = parseInt(url.searchParams.get('offset') || '0')
    return HttpResponse.json({ data: jobs.slice(offset, offset + limit), meta: { total: jobs.length } })
  }),
  http.get(`${BASE}/field-jobs/all`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_FIELD_JOBS, meta: { total: MOCK_FIELD_JOBS.length } })
  }),
  http.patch(`${BASE}/field-jobs/:id/assign`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_FIELD_JOBS[0] })
  }),
  http.post(`${BASE}/field-jobs`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_FIELD_JOBS[0] }, { status: 201 })
  }),

  // ── Water balance ─────────────────────────────────────────
  http.get(`${BASE}/water-balance`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_WATER_BALANCE })
  }),
  http.get(`${BASE}/water-balance/latest`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_WATER_BALANCE })
  }),

  // ── Revenue ───────────────────────────────────────────────
  http.get(`${BASE}/revenue/summary`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_REVENUE_SUMMARY })
  }),
  http.get(`${BASE}/revenue/events`, async () => {
    await d()
    return HttpResponse.json({ data: [], meta: { total: 0 } })
  }),

  // ── Workforce ─────────────────────────────────────────────
  http.get(`${BASE}/workforce/summary`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_WORKFORCE_SUMMARY })
  }),
  http.get(`${BASE}/workforce/active-officers`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_ACTIVE_OFFICERS })
  }),

  // ── NRW Analysis ──────────────────────────────────────────
  http.get(`${BASE}/nrw/summary`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_NRW_SUMMARY, meta: { total: MOCK_NRW_SUMMARY.length } })
  }),
  http.get(`${BASE}/nrw/trend`, async () => {
    await d()
    return HttpResponse.json({ data: [] })
  }),
  http.get(`${BASE}/nrw/my-district`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_NRW_SUMMARY[0] })
  }),

  // ── GRA Compliance ────────────────────────────────────────
  http.get(`${BASE}/gra/compliance`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_AUDIT_EVENTS, meta: { total: MOCK_AUDIT_EVENTS.length } })
  }),
  http.post(`${BASE}/gra/submit/:id`, async () => {
    await d()
    return HttpResponse.json({ data: { success: true } })
  }),

  // ── Reports ───────────────────────────────────────────────
  http.get(`${BASE}/reports/monthly/pdf`, async () => {
    await d(500)
    return new HttpResponse('Mock PDF content', { headers: { 'Content-Type': 'application/pdf' } })
  }),
  http.get(`${BASE}/reports/monthly/csv`, async () => {
    await d(300)
    return new HttpResponse('district,nrw_pct\nAccra Metropolitan,48.1\nKumasi Metropolitan,52.1', { headers: { 'Content-Type': 'text/csv' } })
  }),
  http.get(`${BASE}/reports/gra-compliance/csv`, async () => {
    await d(300)
    return new HttpResponse('audit_ref,account,amount\nAUD-2026-1247,GWL-ACC-004821,48200', { headers: { 'Content-Type': 'text/csv' } })
  }),
  http.get(`${BASE}/reports/audit-trail/csv`, async () => {
    await d(300)
    return new HttpResponse('id,action,user,timestamp\naudit-001,STATUS_CHANGE,admin,2026-03-05', { headers: { 'Content-Type': 'text/csv' } })
  }),
  http.get(`${BASE}/reports/field-jobs/csv`, async () => {
    await d(300)
    return new HttpResponse('job_ref,officer,status\nFJ-2026-0412,Abena Owusu,ON_SITE', { headers: { 'Content-Type': 'text/csv' } })
  }),

  // ── System config ─────────────────────────────────────────
  http.get(`${BASE}/config/:category`, async ({ params }) => {
    await d()
    const cat = (params.category as string).toLowerCase()
    const configs = MOCK_SYSTEM_CONFIG.filter(c => c.category === cat)
    return HttpResponse.json({ data: configs })
  }),
  http.patch(`${BASE}/config/:key`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_SYSTEM_CONFIG[0] })
  }),

  // ── Sentinel ──────────────────────────────────────────────
  http.get(`${BASE}/sentinel/summary`, async () => {
    await d()
    return HttpResponse.json({ data: { total_anomalies: 47, critical: 2, high: 8, medium: 21, low: 16, last_run: '2026-03-06T05:00:00Z' } })
  }),
  http.post(`${BASE}/sentinel/run`, async () => {
    await d(1000)
    return HttpResponse.json({ data: { triggered: 3, message: 'Sentinel run complete' } })
  }),

  // ── DMA Map ───────────────────────────────────────────────
  http.get(`${BASE}/dma/districts`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_DISTRICTS })
  }),
]
