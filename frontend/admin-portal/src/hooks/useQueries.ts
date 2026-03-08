import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import apiClient from '../lib/api-client'
import type { AnomalyFlag, AuditEvent, District, DashboardStats, NRWSummary, ApiResponse } from '../types'

// ============================================================
// DISTRICTS
// ============================================================
export function useDistricts() {
  return useQuery({
    queryKey: ['districts'],
    queryFn: async () => {
      const res = await apiClient.get<ApiResponse<District[]>>('/districts')
      return res.data.data
    },
    staleTime: 5 * 60 * 1000, // 5 minutes
  })
}

export function useDistrict(id: string) {
  return useQuery({
    queryKey: ['districts', id],
    queryFn: async () => {
      const res = await apiClient.get<ApiResponse<District>>(`/districts/${id}`)
      return res.data.data
    },
    enabled: !!id,
  })
}

// ============================================================
// ANOMALY FLAGS
// ============================================================
interface AnomalyFilters {
  district_id?: string
  type?: string
  level?: string
  status?: string
  from?: string
  to?: string
  limit?: number
  offset?: number
}

export function useAnomalies(filters: AnomalyFilters = {}) {
  return useQuery({
    queryKey: ['anomalies', filters],
    queryFn: async () => {
      const params = new URLSearchParams()
      Object.entries(filters).forEach(([k, v]) => {
        if (v !== undefined && v !== '') params.set(k, String(v))
      })
      const res = await apiClient.get<ApiResponse<AnomalyFlag[]>>(
        `/sentinel/anomalies?${params.toString()}`
      )
      return res.data
    },
    refetchInterval: 30 * 1000, // Refresh every 30s
  })
}

export function useAnomaly(id: string) {
  return useQuery({
    queryKey: ['anomalies', id],
    queryFn: async () => {
      const res = await apiClient.get<ApiResponse<AnomalyFlag>>(`/sentinel/anomalies/${id}`)
      return res.data.data
    },
    enabled: !!id,
  })
}

export function useSentinelSummary(districtId: string) {
  return useQuery({
    queryKey: ['sentinel-summary', districtId],
    queryFn: async () => {
      const res = await apiClient.get(`/sentinel/summary/${districtId}`)
      return res.data.data
    },
    enabled: !!districtId,
    refetchInterval: 60 * 1000,
  })
}

// ============================================================
// AUDIT EVENTS
// ============================================================
interface AuditFilters {
  district_id: string
  status?: string
  limit?: number
  offset?: number
}

export function useAuditEvents(filters: AuditFilters) {
  return useQuery({
    queryKey: ['audits', filters],
    queryFn: async () => {
      const params = new URLSearchParams()
      Object.entries(filters).forEach(([k, v]) => {
        if (v !== undefined && v !== '') params.set(k, String(v))
      })
      const res = await apiClient.get<ApiResponse<AuditEvent[]>>(`/audits?${params.toString()}`)
      return res.data
    },
    enabled: !!filters.district_id,
  })
}

export function useAuditEvent(id: string) {
  return useQuery({
    queryKey: ['audits', id],
    queryFn: async () => {
      const res = await apiClient.get<ApiResponse<AuditEvent>>(`/audits/${id}`)
      return res.data.data
    },
    enabled: !!id,
  })
}

export function useDashboardStats(districtId?: string) {
  return useQuery({
    queryKey: ['dashboard-stats', districtId],
    queryFn: async () => {
      const params = districtId ? `?district_id=${districtId}` : ''
      const res = await apiClient.get<ApiResponse<DashboardStats>>(`/audits/dashboard${params}`)
      return res.data.data
    },
    refetchInterval: 60 * 1000,
  })
}

// ============================================================
// NRW SUMMARY
// ============================================================
export function useNRWSummary(districtId?: string) {
  return useQuery({
    queryKey: ['nrw-summary', districtId],
    queryFn: async () => {
      const params = districtId ? `?district_id=${districtId}` : ''
      const res = await apiClient.get<ApiResponse<NRWSummary[]>>(`/reports/nrw${params}`)
      return res.data.data
    },
    staleTime: 5 * 60 * 1000,
  })
}

// ============================================================
// WATER BALANCE (IWA/AWWA M36)
// ============================================================
export function useWaterBalance(districtId?: string) {
  return useQuery({
    queryKey: ['water-balance', districtId],
    queryFn: async () => {
      const params = districtId ? `?district_id=${districtId}` : ''
      const res = await apiClient.get(`/water-balance${params}`)
      return res.data.data as WaterBalanceRecord[]
    },
    staleTime: 5 * 60 * 1000,
  })
}

// ============================================================
// REVENUE RECOVERY (3% success fee)
// ============================================================
export function useRevenueSummary(districtId?: string, period?: string) {
  return useQuery({
    queryKey: ['revenue-summary', districtId, period],
    queryFn: async () => {
      const params = new URLSearchParams()
      if (districtId) params.set('district_id', districtId)
      if (period) params.set('period', period)
      const res = await apiClient.get(`/revenue/summary?${params.toString()}`)
      return res.data.data as RevenueSummary
    },
    staleTime: 2 * 60 * 1000,
  })
}

export function useRevenueEvents(filters: { district_id?: string; status?: string; limit?: number } = {}) {
  return useQuery({
    queryKey: ['revenue-events', filters],
    queryFn: async () => {
      const params = new URLSearchParams()
      Object.entries(filters).forEach(([k, v]) => {
        if (v !== undefined && v !== '') params.set(k, String(v))
      })
      const res = await apiClient.get(`/revenue/events?${params.toString()}`)
      return res.data
    },
  })
}

// ============================================================
// REVENUE LEAKAGE PIPELINE
// ============================================================
// Primary dashboard metric: GHS at each stage of the recovery pipeline.
// Detected → Field Verified → Confirmed → GRA Signed → Collected
export function useLeakagePipeline(districtId?: string) {
  return useQuery({
    queryKey: ['leakage-pipeline', districtId],
    queryFn: async () => {
      const params = districtId ? `?district_id=${districtId}` : ''
      const res = await apiClient.get(`/revenue/pipeline${params}`)
      return res.data.data as LeakagePipeline
    },
    staleTime: 60 * 1000,
    refetchInterval: 2 * 60 * 1000,
  })
}

// ============================================================
// WORKFORCE OVERSIGHT
// ============================================================
export function useWorkforceSummary() {
  return useQuery({
    queryKey: ['workforce-summary'],
    queryFn: async () => {
      const res = await apiClient.get('/workforce/summary')
      return res.data.data as WorkforceSummary
    },
    refetchInterval: 30 * 1000, // refresh every 30s
  })
}

export function useActiveOfficers(districtId?: string) {
  return useQuery({
    queryKey: ['active-officers', districtId],
    queryFn: async () => {
      const params = districtId ? `?district_id=${districtId}` : ''
      const res = await apiClient.get(`/workforce/active${params}`)
      return res.data.data as ActiveOfficer[]
    },
    refetchInterval: 30 * 1000,
  })
}

// ============================================================
// TYPE DEFINITIONS (local to this file)
// ============================================================
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

export interface RevenueSummary {
  total_events: number
  total_variance_ghs: number
  total_recovered_ghs: number
  total_success_fee_ghs: number
  pending_count: number
  confirmed_count: number
  collected_count: number
  by_type: Array<{
    recovery_type: string
    count: number
    recovered_ghs: number
    success_fee_ghs: number
  }>
}

// ============================================================
// REVENUE LEAKAGE MUTATIONS
// ============================================================

// Confirm an anomaly flag as genuine revenue leakage (or false positive)
export function useConfirmAnomaly() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (params: {
      id: string
      confirmed_fraud: boolean
      confirmed_leakage_ghs?: number
      resolution_notes?: string
      leakage_category?: string
    }) => {
      const { id, ...body } = params
      const res = await apiClient.patch(`/sentinel/anomalies/${id}/confirm`, body)
      return res.data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['anomaly-flags'] })
      queryClient.invalidateQueries({ queryKey: ['leakage-pipeline'] })
    },
  })
}

// Record field job outcome (field officer visit result)
export function useRecordFieldJobOutcome() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (params: {
      jobId: string
      outcome: string
      outcome_notes?: string
      meter_found?: boolean
      address_confirmed?: boolean
      recommended_action?: string
      estimated_monthly_m3?: number
    }) => {
      const { jobId, ...body } = params
      const res = await apiClient.patch(`/field-jobs/${jobId}/outcome`, body)
      return res.data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['field-jobs'] })
      queryClient.invalidateQueries({ queryKey: ['anomaly-flags'] })
      queryClient.invalidateQueries({ queryKey: ['leakage-pipeline'] })
    },
  })
}

// Confirm a revenue recovery event (audit manager sets confirmed amount)
export function useConfirmRecovery() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (params: {
      id: string
      recovered_ghs: number
      notes?: string
    }) => {
      const { id, ...body } = params
      const res = await apiClient.patch(`/revenue/events/${id}/confirm`, body)
      return res.data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['revenue-events'] })
      queryClient.invalidateQueries({ queryKey: ['leakage-pipeline'] })
    },
  })
}

// Mark a recovery event as COLLECTED (money physically received)
export function useCollectRecovery() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (params: {
      id: string
      collected_ghs: number
      payment_ref?: string
      collection_notes?: string
    }) => {
      const { id, ...body } = params
      const res = await apiClient.patch(`/revenue/events/${id}/collect`, body)
      return res.data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['revenue-events'] })
      queryClient.invalidateQueries({ queryKey: ['leakage-pipeline'] })
    },
  })
}

export interface LeakagePipelineStage {
  count: number
  ghs: number
}

export interface LeakagePipeline {
  detected: LeakagePipelineStage
  field_verified: LeakagePipelineStage
  confirmed: LeakagePipelineStage
  gra_signed: LeakagePipelineStage
  collected: LeakagePipelineStage
  compliance_flags_open: number
  data_quality_flags_open: number
  total_detected_monthly_ghs: number
  total_detected_annual_ghs: number
  total_collected_ghs: number
  recovery_rate_pct: number
  district_id?: string
}

export interface WorkforceSummary {
  total_field_officers: number
  active_now: number
  on_active_job: number
  idle_officers: number
  jobs_completed_today: number
}

export interface ActiveOfficer {
  officer_id: string
  full_name: string
  employee_id: string
  latitude: number
  longitude: number
  field_job_id?: string
  last_seen_at: string
}
