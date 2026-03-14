/**
 * GN-WAAS Authority Portal — React Query Hooks
 *
 * All data fetching goes through these hooks. No hardcoded mock data.
 * Every hook maps directly to a real API endpoint in the api-gateway.
 */

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import apiClient from '../lib/api-client'
import type {
  WaterAccount,
  District,
  AnomalyFlag,
  AuditEvent,
  FieldJob,
  User,
} from '../types'

// ============================================================
// QUERY KEYS — centralised to avoid typos and enable precise invalidation
// ============================================================
export const QUERY_KEYS = {
  me: ['users', 'me'] as const,
  districts: ['districts'] as const,
  district: (id: string) => ['districts', id] as const,
  myDistrict: ['reports', 'nrw', 'my-district'] as const,
  accounts: (districtId?: string) => ['accounts', districtId] as const,
  accountSearch: (q: string, districtId?: string) => ['accounts', 'search', q, districtId] as const,
  account: (id: string) => ['accounts', id] as const,
  nrwSummary: (districtId?: string, period?: string) => ['reports', 'nrw', districtId, period] as const,
  nrwTrend: (districtId: string) => ['reports', 'nrw', districtId, 'trend'] as const,
  anomalyFlags: (districtId?: string) => ['anomaly-flags', districtId] as const,
  auditEvents: (districtId?: string) => ['audits', districtId] as const,
  auditEvent: (id: string) => ['audits', id] as const,
  dashboardStats: ['audits', 'dashboard'] as const,
  myJobs: ['field-jobs', 'my-jobs'] as const,
  fieldOfficers: (districtId?: string) => ['users', 'field-officers', districtId] as const,
  allFieldJobs: (status?: string, districtId?: string) => ['field-jobs', 'all', status, districtId] as const,
  monthlyReport: (period: string, districtId?: string) => ['reports', 'monthly', period, districtId] as const,
  sentinelSummary: (districtId: string) => ['sentinel', 'summary', districtId] as const,
} as const

// ============================================================
// RESPONSE TYPES
// ============================================================

interface PaginatedResponse<T> {
  data: T[]
  meta: {
    total: number
    page: number
    page_size: number
  }
}

interface NRWSummaryRow {
  district_id: string
  district_code: string
  district_name: string
  region: string
  period_start: string
  period_end: string
  total_accounts: number
  flagged_accounts: number
  open_anomalies: number
  critical_anomalies: number
  high_anomalies: number
  total_estimated_loss_ghs: number
  total_confirmed_loss_ghs: number
  total_recovered_ghs: number
  loss_ratio_pct?: number
  data_confidence_grade?: number
  is_pilot_district: boolean
  grade: string
  zone_type?: string
}

interface NRWTrendPoint {
  month: string
  open_flags: number
  resolved_flags: number
  estimated_loss_ghs: number
}

interface DashboardStats {
  total_open_anomalies: number
  critical_anomalies: number
  high_anomalies: number
  total_estimated_loss_ghs: number
  total_confirmed_loss_ghs: number
  total_recovered_ghs: number
  audits_in_progress: number
  audits_completed_this_month: number
  field_jobs_active: number
}

interface MyDistrictResponse {
  district: District
  summary: NRWSummaryRow | null
}

// ============================================================
// USER HOOKS
// ============================================================

/** Fetch the currently authenticated user's profile */
export function useMe() {
  return useQuery<User>({
    queryKey: QUERY_KEYS.me,
    queryFn: async () => {
      const { data } = await apiClient.get<{ data: User }>('/users/me')
      return data.data
    },
    staleTime: 5 * 60 * 1000, // 5 minutes
    retry: 1,
  })
}

/** Fetch field officers, optionally filtered by district */
export function useFieldOfficers(districtId?: string) {
  return useQuery<User[]>({
    queryKey: QUERY_KEYS.fieldOfficers(districtId),
    queryFn: async () => {
      const params = districtId ? { district_id: districtId } : {}
      const { data } = await apiClient.get<{ data: User[] }>('/users/field-officers', { params })
      return data.data ?? []
    },
    staleTime: 2 * 60 * 1000,
  })
}

// ============================================================
// DISTRICT HOOKS
// ============================================================

/** Fetch all active districts */
export function useDistricts() {
  return useQuery<District[]>({
    queryKey: QUERY_KEYS.districts,
    queryFn: async () => {
      const { data } = await apiClient.get<{ data: District[] }>('/districts')
      return data.data ?? []
    },
    staleTime: 10 * 60 * 1000, // Districts rarely change
  })
}

/** Fetch a single district by ID */
export function useDistrict(id: string) {
  return useQuery<District>({
    queryKey: QUERY_KEYS.district(id),
    queryFn: async () => {
      const { data } = await apiClient.get<{ data: District }>(`/districts/${id}`)
      return data.data
    },
    enabled: !!id,
    staleTime: 10 * 60 * 1000,
  })
}

/** Fetch the authenticated user's district summary (for GWL staff portal) */
export function useMyDistrict() {
  return useQuery<MyDistrictResponse>({
    queryKey: QUERY_KEYS.myDistrict,
    queryFn: async () => {
      const { data } = await apiClient.get<{ data: MyDistrictResponse }>('/reports/nrw/my-district')
      return data.data
    },
    staleTime: 2 * 60 * 1000,
  })
}

// ============================================================
// WATER ACCOUNT HOOKS
// ============================================================

/** Search water accounts by query string and optional district */
export function useAccountSearch(query: string, districtId?: string, enabled = true) {
  return useQuery<{ accounts: WaterAccount[]; total: number }>({
    queryKey: QUERY_KEYS.accountSearch(query, districtId),
    queryFn: async () => {
      const params: Record<string, string> = { q: query }
      if (districtId) params.district_id = districtId
      const { data } = await apiClient.get<PaginatedResponse<WaterAccount>>('/accounts/search', { params })
      return { accounts: data.data ?? [], total: data.meta?.total ?? 0 }
    },
    enabled: enabled && query.length >= 2,
    staleTime: 30 * 1000, // 30 seconds
  })
}

/** Fetch accounts for a specific district */
export function useAccountsByDistrict(districtId: string, page = 1, pageSize = 50) {
  return useQuery<{ accounts: WaterAccount[]; total: number }>({
    queryKey: [...QUERY_KEYS.accounts(districtId), page, pageSize],
    queryFn: async () => {
      const offset = (page - 1) * pageSize
      const { data } = await apiClient.get<PaginatedResponse<WaterAccount>>('/accounts', {
        params: { district_id: districtId, limit: pageSize, offset },
      })
      return { accounts: data.data ?? [], total: data.meta?.total ?? 0 }
    },
    enabled: !!districtId,
    staleTime: 60 * 1000,
  })
}

/** Fetch a single water account by ID */
export function useAccount(id: string) {
  return useQuery<WaterAccount>({
    queryKey: QUERY_KEYS.account(id),
    queryFn: async () => {
      const { data } = await apiClient.get<{ data: WaterAccount }>(`/accounts/${id}`)
      return data.data
    },
    enabled: !!id,
    staleTime: 60 * 1000,
  })
}

// ============================================================
// NRW REPORT HOOKS
// ============================================================

/** Fetch NRW summary for all districts (or a specific one) */
export function useNRWSummary(districtId?: string, period?: string) {
  return useQuery<NRWSummaryRow[]>({
    queryKey: QUERY_KEYS.nrwSummary(districtId, period),
    queryFn: async () => {
      const params: Record<string, string> = {}
      if (districtId) params.district_id = districtId
      if (period) {
        // Translate UI period shorthand to from/to date strings
        const now = new Date()
        const to = now.toISOString().split('T')[0]
        const days = period === '30d' ? 30 : period === '90d' ? 90 : period === '6m' ? 180 : 365
        const from = new Date(now.getTime() - days * 86400_000).toISOString().split('T')[0]
        params.from = from
        params.to   = to
      }
      const { data } = await apiClient.get<{ data: NRWSummaryRow[] }>('/reports/nrw', { params })
      return data.data ?? []
    },
    staleTime: 5 * 60 * 1000,
  })
}

/** Fetch 12-month NRW trend for a specific district */
export function useNRWTrend(districtId: string) {
  return useQuery<NRWTrendPoint[]>({
    queryKey: QUERY_KEYS.nrwTrend(districtId),
    queryFn: async () => {
      const { data } = await apiClient.get<{ data: NRWTrendPoint[] }>(
        `/reports/nrw/${districtId}/trend`
      )
      return data.data ?? []
    },
    enabled: !!districtId,
    staleTime: 5 * 60 * 1000,
  })
}

// ============================================================
// AUDIT EVENT HOOKS
// ============================================================

/** Fetch dashboard statistics */
export function useDashboardStats() {
  return useQuery<DashboardStats>({
    queryKey: QUERY_KEYS.dashboardStats,
    queryFn: async () => {
      const { data } = await apiClient.get<{ data: DashboardStats }>('/audits/dashboard')
      return data.data
    },
    staleTime: 60 * 1000,
    refetchInterval: 2 * 60 * 1000, // Auto-refresh every 2 minutes
  })
}

/** Fetch audit events with optional district filter */
export function useAuditEvents(districtId?: string, page = 1, pageSize = 20) {
  return useQuery<{ events: AuditEvent[]; total: number }>({
    queryKey: [...QUERY_KEYS.auditEvents(districtId), page, pageSize],
    queryFn: async () => {
      const offset = (page - 1) * pageSize
      const params: Record<string, string | number> = { limit: pageSize, offset }
      if (districtId) params.district_id = districtId
      const { data } = await apiClient.get<PaginatedResponse<AuditEvent>>('/audits', { params })
      return { events: data.data ?? [], total: data.meta?.total ?? 0 }
    },
    staleTime: 30 * 1000,
  })
}

/** Fetch a single audit event by ID */
export function useAuditEvent(id: string) {
  return useQuery<AuditEvent>({
    queryKey: QUERY_KEYS.auditEvent(id),
    queryFn: async () => {
      const { data } = await apiClient.get<{ data: AuditEvent }>(`/audits/${id}`)
      return data.data
    },
    enabled: !!id,
    staleTime: 30 * 1000,
  })
}

// ============================================================
// FIELD JOB HOOKS
// ============================================================

/** Fetch the authenticated field officer's assigned jobs */
export function useMyJobs() {
  return useQuery<FieldJob[]>({
    queryKey: QUERY_KEYS.myJobs,
    queryFn: async () => {
      const { data } = await apiClient.get<{ data: FieldJob[] }>('/field-jobs/my-jobs')
      return data.data ?? []
    },
    staleTime: 30 * 1000,
    refetchInterval: 60 * 1000, // Poll every minute for new jobs
  })
}

/** Update a field job's status */
export function useUpdateJobStatus() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({
      jobId,
      status,
      gpsLat,
      gpsLng,
    }: {
      jobId: string
      status: string
      gpsLat?: number
      gpsLng?: number
    }) => {
      const { data } = await apiClient.patch(`/field-jobs/${jobId}/status`, {
        status,
        gps_lat: gpsLat,
        gps_lng: gpsLng,
      })
      return data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.myJobs })
    },
  })
}

/** Trigger SOS for a field job */
export function useTriggerSOS() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({
      jobId,
      gpsLat,
      gpsLng,
    }: {
      jobId: string
      gpsLat: number
      gpsLng: number
    }) => {
      const { data } = await apiClient.post(`/field-jobs/${jobId}/sos`, {
        gps_lat: gpsLat,
        gps_lng: gpsLng,
      })
      return data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.myJobs })
    },
  })
}

// ============================================================
// ANOMALY FLAG HOOKS
// ============================================================

/** Fetch anomaly flags for a district */
export function useAnomalyFlags(districtId?: string) {
  return useQuery<AnomalyFlag[]>({
    queryKey: QUERY_KEYS.anomalyFlags(districtId),
    queryFn: async () => {
      const params = districtId ? { district_id: districtId } : {}
      const { data } = await apiClient.get<{ data: AnomalyFlag[] }>('/anomaly-flags', { params })
      return data.data ?? []
    },
    staleTime: 60 * 1000,
  })
}

// ============================================================
// FIELD JOB MANAGEMENT HOOKS (Authority Portal — admin view)
// ============================================================

/** Fetch ALL field jobs (admin/supervisor view) */
export function useAllFieldJobs(status?: string, districtId?: string) {
  return useQuery<FieldJob[]>({
    queryKey: QUERY_KEYS.allFieldJobs(status, districtId),
    queryFn: async () => {
      const params: Record<string, string> = {}
      if (status) params.status = status
      if (districtId) params.district_id = districtId
      const { data } = await apiClient.get<{ data: FieldJob[] }>('/field-jobs', { params })
      return data.data ?? []
    },
    staleTime: 30 * 1000,
    refetchInterval: 30 * 1000, // auto-refresh every 30s
  })
}

/** Assign a field officer to a job */
export function useAssignOfficer() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ jobId, officerId }: { jobId: string; officerId: string }) => {
      const { data } = await apiClient.patch(`/field-jobs/${jobId}/assign`, {
        officer_id: officerId,
      })
      return data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['field-jobs'] })
    },
  })
}

/** Fetch field officers for a district */
export function useFieldOfficersList(districtId?: string) {
  return useQuery<User[]>({
    queryKey: QUERY_KEYS.fieldOfficers(districtId),
    queryFn: async () => {
      const params = districtId ? { district_id: districtId } : {}
      const { data } = await apiClient.get<{ data: User[] }>('/users/field-officers', { params })
      return data.data ?? []
    },
    staleTime: 5 * 60 * 1000,
  })
}

/** Fetch sentinel district summary */
export function useSentinelSummary(districtId: string) {
  return useQuery({
    queryKey: QUERY_KEYS.sentinelSummary(districtId),
    queryFn: async () => {
      const { data } = await apiClient.get(`/sentinel/summary/${districtId}`)
      return data
    },
    enabled: !!districtId,
    staleTime: 60 * 1000,
  })
}

// ============================================================
// EXPORT TYPES for use in pages
// ============================================================
export type { NRWSummaryRow, NRWTrendPoint, DashboardStats, MyDistrictResponse }

// ── Water Balance (IWA/AWWA M36) ──────────────────────────────────────────────
export function useWaterBalance(districtId?: string) {
  return useQuery({
    queryKey: ['water-balance', districtId],
    queryFn: async () => {
      const params = districtId ? `?district_id=${districtId}` : ''
      const { data } = await apiClient.get(`/water-balance${params}`)
      return data.data as WaterBalanceRecord[]
    },
    staleTime: 5 * 60 * 1000,
    enabled: !!districtId,
  })
}

export interface WaterBalanceRecord {
  district_id: string
  period_start: string
  period_end: string
  system_input_m3: number
  billed_metered_m3: number
  billed_unmetered_m3: number
  unbilled_metered_m3: number
  unbilled_unmetered_m3: number
  total_authorised_m3: number
  unauthorised_consumption_m3: number
  metering_inaccuracies_m3: number
  data_handling_errors_m3: number
  total_apparent_losses_m3: number
  main_leakage_m3: number
  storage_overflow_m3: number
  service_connection_leak_m3: number
  total_real_losses_m3: number
  total_water_losses_m3: number
  nrw_m3: number
  nrw_percent: number
  ili: number
  iwa_grade: string
  estimated_revenue_recovery_ghs: number
  data_confidence_score: number
  computed_at: string
}

// ============================================================
// ANOMALY FLAG ACTIONS
// ============================================================

/** Confirm or dismiss an anomaly flag.
 *  Maps to PATCH /api/v1/sentinel/anomalies/:id/confirm
 */
export function useConfirmAnomaly() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({
      id,
      confirmedFraud,
      resolutionNotes,
    }: {
      id: string
      confirmedFraud: boolean
      resolutionNotes?: string
    }) => {
      const { data } = await apiClient.patch(`/sentinel/anomalies/${id}/confirm`, {
        confirmed_fraud: confirmedFraud,
        resolution_notes: resolutionNotes ?? '',
      })
      return data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['anomaly-flags'] })
    },
  })
}

/** Move an anomaly to INVESTIGATING status (acknowledge it).
 *  Uses a generic status-update endpoint PATCH /api/v1/anomaly-flags/:id/status
 */
export function useUpdateAnomalyStatus() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, status, notes }: { id: string; status: string; notes?: string }) => {
      const { data } = await apiClient.patch(`/anomaly-flags/${id}/status`, {
        status,
        resolution_notes: notes ?? '',
      })
      return data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['anomaly-flags'] })
    },
  })
}
