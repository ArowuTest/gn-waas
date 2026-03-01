import { useQuery } from '@tanstack/react-query'
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
