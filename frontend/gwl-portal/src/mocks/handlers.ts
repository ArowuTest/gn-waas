import { http, HttpResponse, delay } from 'msw'
import {
  MOCK_GWL_TOKEN, MOCK_GWL_USER, MOCK_CASE_SUMMARY, MOCK_CASES,
  MOCK_CASE_ACTIONS, MOCK_RECLASSIFICATIONS, MOCK_CREDITS,
  MOCK_FIELD_OFFICERS, MOCK_MONTHLY_REPORT,
} from './mockData'

const BASE = '/api/v1'
const d = (ms = 200) => delay(ms)

const MOCK_DISTRICTS = [
  { id: 'dist-001', district_name: 'Accra Metropolitan' },
  { id: 'dist-002', district_name: 'Kumasi Metropolitan' },
  { id: 'dist-003', district_name: 'Tema Municipal' },
  { id: 'dist-004', district_name: 'Tamale Metropolitan' },
  { id: 'dist-005', district_name: 'Cape Coast Municipal' },
]

export const handlers = [
  // ── Auth ──────────────────────────────────────────────────
  http.post(`${BASE}/auth/dev-login`, async () => {
    await d()
    return HttpResponse.json({ data: { access_token: MOCK_GWL_TOKEN, user: MOCK_GWL_USER } })
  }),
  http.post(`${BASE}/auth/login`, async () => {
    await d()
    return HttpResponse.json({ data: { access_token: MOCK_GWL_TOKEN, user: MOCK_GWL_USER } })
  }),
  http.post(`${BASE}/auth/refresh`, async () => {
    await d()
    return HttpResponse.json({ data: { access_token: MOCK_GWL_TOKEN } })
  }),

  // ── Current user ──────────────────────────────────────────
  http.get(`${BASE}/users/me`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_GWL_USER })
  }),

  // ── Districts ─────────────────────────────────────────────
  http.get(`${BASE}/districts`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_DISTRICTS })
  }),

  // ── GWL Case Management ───────────────────────────────────
  http.get(`${BASE}/gwl/cases/summary`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_CASE_SUMMARY })
  }),

  http.get(`${BASE}/gwl/cases`, async ({ request }) => {
    await d()
    const url = new URL(request.url)
    const status = url.searchParams.get('gwl_status') || ''
    const severity = url.searchParams.get('severity') || ''
    const flagType = url.searchParams.get('flag_type') || ''
    let cases = [...MOCK_CASES]
    if (status) cases = cases.filter(c => c.gwl_status === status)
    if (severity) cases = cases.filter(c => c.alert_level === severity)
    if (flagType) cases = cases.filter(c => c.anomaly_type === flagType)
    const limit = parseInt(url.searchParams.get('limit') || '50')
    const offset = parseInt(url.searchParams.get('offset') || '0')
    return HttpResponse.json({ data: cases.slice(offset, offset + limit), meta: { total: cases.length } })
  }),

  http.get(`${BASE}/gwl/cases/:id`, async ({ params }) => {
    await d()
    const c = MOCK_CASES.find(x => x.id === params.id) || MOCK_CASES[0]
    return HttpResponse.json({ data: c })
  }),

  http.get(`${BASE}/gwl/cases/:id/actions`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_CASE_ACTIONS })
  }),

  http.post(`${BASE}/gwl/cases/:id/assign`, async () => {
    await d()
    return HttpResponse.json({ data: { ...MOCK_CASES[0], gwl_status: 'FIELD_ASSIGNED', assigned_officer: 'Abena Owusu' } })
  }),

  http.patch(`${BASE}/gwl/cases/:id/status`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_CASES[0] })
  }),

  http.post(`${BASE}/gwl/cases/:id/reclassify`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_RECLASSIFICATIONS[0] }, { status: 201 })
  }),

  http.post(`${BASE}/gwl/cases/:id/credit`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_CREDITS[0] }, { status: 201 })
  }),

  // ── Reclassifications ─────────────────────────────────────
  http.get(`${BASE}/gwl/reclassifications`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_RECLASSIFICATIONS, meta: { total: MOCK_RECLASSIFICATIONS.length } })
  }),

  // ── Credits ───────────────────────────────────────────────
  http.get(`${BASE}/gwl/credits`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_CREDITS, meta: { total: MOCK_CREDITS.length } })
  }),

  // ── Reports ───────────────────────────────────────────────
  http.get(`${BASE}/gwl/reports/monthly`, async () => {
    await d(400)
    return HttpResponse.json({ data: MOCK_MONTHLY_REPORT })
  }),

  // ── Revenue ───────────────────────────────────────────────
  http.get(`${BASE}/revenue/summary`, async () => {
    await d()
    return HttpResponse.json({ data: { total_events: 1274, total_variance_ghs: 8420000, total_recovered_ghs: 1842650, total_success_fee_ghs: 55279.50, pending_count: 47, confirmed_count: 312, collected_count: 891, by_type: [] } })
  }),

  // ── Field officers ────────────────────────────────────────
  http.get(`${BASE}/users/field-officers`, async () => {
    await d()
    return HttpResponse.json({ data: MOCK_FIELD_OFFICERS })
  }),

  // ── Anomaly flags (for report issue) ─────────────────────
  http.post(`${BASE}/anomaly-flags`, async () => {
    await d()
    return HttpResponse.json({ data: { id: 'new-flag-' + Date.now(), flag_reference: 'ANF-2026-0900' } }, { status: 201 })
  }),
]
