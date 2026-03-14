import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '../lib/api-client'
import { useDistricts } from '../hooks/useQueries'
import {
  MapPin, User, Clock, AlertTriangle, CheckCircle,
  RefreshCw, Eye, Navigation, Filter, Search
} from 'lucide-react'

// ─── Types ────────────────────────────────────────────────────────────────────

interface FieldJob {
  id: string
  job_reference: string
  account_number: string
  customer_name: string
  address: string
  gps_lat: number
  gps_lng: number
  anomaly_type: string
  alert_level: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'INFO'
  status: 'QUEUED' | 'DISPATCHED' | 'EN_ROUTE' | 'ON_SITE' | 'COMPLETED' | 'FAILED' | 'SOS'
  assigned_officer_id?: string
  assigned_officer_name?: string
  scheduled_at?: string
  dispatched_at?: string
  completed_at?: string
  estimated_variance_ghs?: number
  notes?: string
  district_name?: string
}

interface FieldOfficer {
  id: string
  full_name: string
  email: string
  badge_number?: string
  district_id: string
  district_name?: string
  active_jobs: number
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

const STATUS_STYLES: Record<string, string> = {
  QUEUED:     'bg-gray-100 text-gray-700',
  DISPATCHED: 'bg-blue-100 text-blue-700',
  EN_ROUTE:   'bg-indigo-100 text-indigo-700',
  ON_SITE:    'bg-yellow-100 text-yellow-700',
  COMPLETED:  'bg-green-100 text-green-700',
  FAILED:     'bg-red-100 text-red-700',
  SOS:        'bg-red-600 text-white animate-pulse',
}

const ALERT_STYLES: Record<string, string> = {
  CRITICAL: 'text-red-600 font-bold',
  HIGH:     'text-orange-500 font-semibold',
  MEDIUM:   'text-yellow-600',
  LOW:      'text-blue-500',
  INFO:     'text-gray-400',
}

function formatDate(iso?: string) {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('en-GB', {
    day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit',
  })
}

// ─── Assign Officer Modal ─────────────────────────────────────────────────────

function AssignModal({
  job,
  officers,
  onAssign,
  onClose,
}: {
  job: FieldJob
  officers: FieldOfficer[]
  onAssign: (jobId: string, officerId: string) => void
  onClose: () => void
}) {
  const [selected, setSelected] = useState(job.assigned_officer_id ?? '')

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
      <div className="bg-white rounded-xl shadow-xl w-full max-w-md p-6">
        <h3 className="text-lg font-semibold text-gray-900 mb-1">Assign Field Officer</h3>
        <p className="text-sm text-gray-500 mb-4">
          Job <span className="font-mono font-medium">{job.job_reference}</span> — {job.customer_name}
        </p>

        <div className="space-y-2 max-h-64 overflow-y-auto">
          {officers.map(o => (
            <label
              key={o.id}
              className={`flex items-center gap-3 p-3 rounded-lg border cursor-pointer transition-colors ${
                selected === o.id
                  ? 'border-brand-500 bg-brand-50'
                  : 'border-gray-200 hover:border-gray-300'
              }`}
            >
              <input
                type="radio"
                name="officer"
                value={o.id}
                checked={selected === o.id}
                onChange={() => setSelected(o.id)}
                className="accent-brand-600"
              />
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-gray-900">{o.full_name}</p>
                <p className="text-xs text-gray-400">
                  {o.badge_number && <span className="mr-2">#{o.badge_number}</span>}
                  {o.district_name} · {o.active_jobs} active job{o.active_jobs !== 1 ? 's' : ''}
                </p>
              </div>
              {o.active_jobs >= 5 && (
                <span className="text-xs text-orange-500 font-medium">At capacity</span>
              )}
            </label>
          ))}
          {officers.length === 0 && (
            <p className="text-sm text-gray-400 text-center py-4">No field officers available</p>
          )}
        </div>

        <div className="flex gap-3 mt-5">
          <button
            onClick={onClose}
            className="flex-1 px-4 py-2 border border-gray-200 rounded-lg text-sm text-gray-600 hover:bg-gray-50"
          >
            Cancel
          </button>
          <button
            onClick={() => { if (selected) { onAssign(job.id, selected); onClose() } }}
            disabled={!selected}
            className="flex-1 px-4 py-2 bg-brand-600 text-white rounded-lg text-sm font-medium hover:bg-brand-700 disabled:opacity-40"
          >
            Assign
          </button>
        </div>
      </div>
    </div>
  )
}

// ─── Job Row ──────────────────────────────────────────────────────────────────

function JobRow({
  job,
  onAssign,
  onViewMap,
}: {
  job: FieldJob
  onAssign: (job: FieldJob) => void
  onViewMap: (job: FieldJob) => void
}) {
  return (
    <tr className="hover:bg-gray-50 transition-colors">
      <td className="px-4 py-3">
        <div className="flex flex-col">
          <span className="font-mono text-xs text-gray-500">{job.job_reference}</span>
          <span className="text-sm font-medium text-gray-900">{job.customer_name}</span>
          <span className="text-xs text-gray-400 truncate max-w-[180px]">{job.address}</span>
        </div>
      </td>
      <td className="px-4 py-3">
        <span className={`text-xs font-medium ${ALERT_STYLES[job.alert_level ?? 'INFO'] ?? ''}`}>
          {job.alert_level}
        </span>
        <p className="text-xs text-gray-400 mt-0.5">{(job.anomaly_type ?? '').replace(/_/g, ' ')}</p>
      </td>
      <td className="px-4 py-3">
        <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${STATUS_STYLES[job.status] ?? 'bg-gray-100 text-gray-700'}`}>
          {job.status === 'SOS' && '🚨 '}
          {job.status.replace(/_/g, ' ')}
        </span>
      </td>
      <td className="px-4 py-3">
        {job.assigned_officer_name ? (
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 bg-brand-100 rounded-full flex items-center justify-center flex-shrink-0">
              <span className="text-brand-700 text-xs font-bold">
                {job.assigned_officer_name.charAt(0)}
              </span>
            </div>
            <span className="text-sm text-gray-700">{job.assigned_officer_name}</span>
          </div>
        ) : (
          <span className="text-xs text-gray-400 italic">Unassigned</span>
        )}
      </td>
      <td className="px-4 py-3 text-xs text-gray-500">
        {formatDate(job.dispatched_at ?? job.scheduled_at)}
      </td>
      <td className="px-4 py-3">
        {job.estimated_variance_ghs != null && (
          <span className="text-sm font-medium text-red-600">
            ₵{job.estimated_variance_ghs.toFixed(2)}
          </span>
        )}
      </td>
      <td className="px-4 py-3">
        <div className="flex items-center gap-2">
          <button
            onClick={() => onAssign(job)}
            className="p-1.5 text-gray-400 hover:text-brand-600 hover:bg-brand-50 rounded transition-colors"
            title="Assign officer"
          >
            <User size={14} />
          </button>
          <button
            onClick={() => onViewMap(job)}
            className="p-1.5 text-gray-400 hover:text-blue-600 hover:bg-blue-50 rounded transition-colors"
            title="View on map"
          >
            <Navigation size={14} />
          </button>
        </div>
      </td>
    </tr>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export function FieldJobsPage() {
  const queryClient = useQueryClient()
  const [statusFilter, setStatusFilter] = useState<string>('ALL')
  const [alertFilter, setAlertFilter]   = useState<string>('ALL')
  const [search, setSearch]             = useState('')
  const [assigningJob, setAssigningJob] = useState<FieldJob | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [newJob, setNewJob] = useState({ account_id: '', district_id: '', priority: 2, notes: '' })
  const [createError, setCreateError] = useState('')
  // UX-1: account search state for the Dispatch Job modal
  const [accountQuery, setAccountQuery] = useState('')
  const [accountSearchResults, setAccountSearchResults] = useState<Array<{ id: string; gwl_account_number: string; account_holder_name: string }>>([])
  const [accountSearching, setAccountSearching] = useState(false)

  const { data: districtsData } = useDistricts()

  const createJobMutation = useMutation({
    mutationFn: async (payload: typeof newJob) => {
      await apiClient.post('/field-jobs', {
        account_id: payload.account_id || undefined,
        district_id: payload.district_id || undefined,
        priority: payload.priority,
        notes: payload.notes || undefined,
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['field-jobs'] })
      setShowCreate(false)
      setNewJob({ account_id: '', district_id: '', priority: 2, notes: '' })
      setAccountQuery('')
      setAccountSearchResults([])
      setCreateError('')
    },
    onError: (err: any) => {
      setCreateError(err.response?.data?.error?.message || err.response?.data?.error || 'Failed to create job')
    },
  })

  // UX-1: debounced account search for the dispatch modal
  const handleAccountSearch = async (q: string) => {
    setAccountQuery(q)
    if (q.length < 2) { setAccountSearchResults([]); return }
    setAccountSearching(true)
    try {
      const res = await apiClient.get('/accounts/search', { params: { q, limit: 8 } })
      setAccountSearchResults(res.data?.data ?? res.data?.accounts ?? [])
    } catch {
      setAccountSearchResults([])
    } finally {
      setAccountSearching(false)
    }
  }

  // ── Data fetching ──────────────────────────────────────────────────────────
  const { data: jobsData, isLoading: jobsLoading, refetch } = useQuery({
    queryKey: ['field-jobs', statusFilter, alertFilter],
    queryFn: async () => {
      const params = new URLSearchParams()
      if (statusFilter !== 'ALL') params.set('status', statusFilter)
      if (alertFilter  !== 'ALL') params.set('alert_level', alertFilter)
      const res = await apiClient.get(`/field-jobs?${params}`)
      return res.data.data as FieldJob[]
    },
    refetchInterval: 30_000, // auto-refresh every 30s
  })

  const { data: officersData } = useQuery({
    queryKey: ['field-officers'],
    queryFn: async () => {
      try {
        const res = await apiClient.get('/users/field-officers')
        return (res.data.data ?? []) as FieldOfficer[]
      } catch {
        return [] as FieldOfficer[]
      }
    },
    retry: false,
  })

  const assignMutation = useMutation({
    mutationFn: async ({ jobId, officerId }: { jobId: string; officerId: string }) => {
      await apiClient.patch(`/field-jobs/${jobId}/assign`, { officer_id: officerId })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['field-jobs'] })
      queryClient.invalidateQueries({ queryKey: ['field-officers'] })
    },
  })

  // ── Derived data ───────────────────────────────────────────────────────────
  const jobs = jobsData ?? []
  const officers = officersData ?? []

  const filtered = jobs.filter(j => {
    if (search) {
      const q = search.toLowerCase()
      if (!j.customer_name.toLowerCase().includes(q) &&
          !j.job_reference.toLowerCase().includes(q) &&
          !j.account_number.toLowerCase().includes(q)) return false
    }
    return true
  })

  const sosJobs      = jobs.filter(j => j.status === 'SOS')
  const activeJobs   = jobs.filter(j => ['DISPATCHED', 'EN_ROUTE', 'ON_SITE'].includes(j.status))
  const queuedJobs   = jobs.filter(j => j.status === 'QUEUED')
  const criticalJobs = jobs.filter(j => j.alert_level === 'CRITICAL')

  const handleViewMap = (job: FieldJob) => {
    window.open(`https://maps.google.com/?q=${job.gps_lat},${job.gps_lng}`, '_blank')
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Field Jobs</h1>
          <p className="text-sm text-gray-500 mt-0.5">
            Manage and monitor field officer assignments in real time
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setShowCreate(true)}
            className="flex items-center gap-2 px-4 py-2 bg-green-700 text-white rounded-lg text-sm font-medium hover:bg-green-800"
          >
            + Dispatch Job
          </button>
          <button
            onClick={() => refetch()}
            className="flex items-center gap-2 px-4 py-2 bg-brand-600 text-white rounded-lg text-sm font-medium hover:bg-brand-700"
          >
            <RefreshCw size={14} />
            Refresh
          </button>
        </div>
      </div>

      {/* Create Job Modal */}
      {showCreate && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-2xl p-6 w-full max-w-md shadow-xl">
            <h3 className="font-bold text-gray-900 mb-4">Dispatch New Field Job</h3>
            <div className="space-y-3 mb-4">
              <div>
                <label className="block text-xs font-semibold text-gray-600 mb-1">
                  Account (optional)
                  {newJob.account_id && <span className="text-green-600 ml-1">✓ selected</span>}
                </label>
                <input
                  type="text"
                  value={accountQuery}
                  onChange={e => handleAccountSearch(e.target.value)}
                  placeholder="Search by account number or name…"
                  className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm"
                />
                {accountSearching && (
                  <p className="text-xs text-gray-400 mt-1">Searching…</p>
                )}
                {accountSearchResults.length > 0 && (
                  <ul className="mt-1 border border-gray-200 rounded-lg overflow-hidden text-sm shadow">
                    {accountSearchResults.map(a => (
                      <li
                        key={a.id}
                        className="px-3 py-2 hover:bg-green-50 cursor-pointer flex items-center justify-between"
                        onClick={() => {
                          setNewJob(j => ({ ...j, account_id: a.id }))
                          setAccountQuery(`${a.gwl_account_number} — ${a.account_holder_name}`)
                          setAccountSearchResults([])
                        }}
                      >
                        <span className="font-medium">{a.account_holder_name}</span>
                        <span className="text-gray-400 text-xs font-mono">{a.gwl_account_number}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
              <div>
                <label className="block text-xs font-semibold text-gray-600 mb-1">District (optional)</label>
                <select
                  value={newJob.district_id}
                  onChange={e => setNewJob(j => ({ ...j, district_id: e.target.value }))}
                  className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm"
                >
                  <option value="">— All districts / unassigned —</option>
                  {(districtsData ?? []).map((d: any) => (
                    <option key={d.id} value={d.id}>{d.district_name}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-xs font-semibold text-gray-600 mb-1">Priority</label>
                <select
                  value={newJob.priority}
                  onChange={e => setNewJob(j => ({ ...j, priority: parseInt(e.target.value) }))}
                  className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm"
                >
                  <option value={1}>1 — Critical</option>
                  <option value={2}>2 — High</option>
                  <option value={3}>3 — Medium</option>
                  <option value={4}>4 — Low</option>
                </select>
              </div>
              <div>
                <label className="block text-xs font-semibold text-gray-600 mb-1">Notes (optional)</label>
                <textarea
                  rows={2}
                  value={newJob.notes}
                  onChange={e => setNewJob(j => ({ ...j, notes: e.target.value }))}
                  placeholder="Dispatch instructions..."
                  className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm resize-none"
                />
              </div>
            </div>
            {createError && (
              <div className="mb-3 p-2 bg-red-50 border border-red-200 rounded text-red-700 text-xs">{createError}</div>
            )}
            <div className="flex gap-2">
              <button
                onClick={() => { setShowCreate(false); setCreateError('') }}
                className="flex-1 border border-gray-200 text-gray-600 font-semibold py-2 rounded-lg text-sm"
              >
                Cancel
              </button>
              <button
                onClick={() => createJobMutation.mutate(newJob)}
                disabled={createJobMutation.isPending}
                className="flex-1 bg-green-700 text-white font-bold py-2 rounded-lg text-sm disabled:opacity-50"
              >
                {createJobMutation.isPending ? 'Dispatching...' : 'Dispatch'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* SOS Alert Banner */}
      {sosJobs.length > 0 && (
        <div className="bg-red-600 text-white rounded-xl p-4 flex items-center gap-3 animate-pulse">
          <AlertTriangle size={20} />
          <div>
            <p className="font-bold">🚨 {sosJobs.length} SOS Alert{sosJobs.length > 1 ? 's' : ''} Active</p>
            <p className="text-sm text-red-100">
              {sosJobs.map(j => j.assigned_officer_name ?? j.job_reference).join(', ')} — immediate response required
            </p>
          </div>
        </div>
      )}

      {/* KPI Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        {[
          { label: 'SOS Active',    value: sosJobs.length,    icon: <AlertTriangle size={18} />, color: 'text-red-600',    bg: 'bg-red-50' },
          { label: 'In Progress',   value: activeJobs.length, icon: <Navigation size={18} />,    color: 'text-blue-600',   bg: 'bg-blue-50' },
          { label: 'Queued',        value: queuedJobs.length, icon: <Clock size={18} />,          color: 'text-yellow-600', bg: 'bg-yellow-50' },
          { label: 'Critical Jobs', value: criticalJobs.length, icon: <Eye size={18} />,          color: 'text-orange-600', bg: 'bg-orange-50' },
        ].map(card => (
          <div key={card.label} className="bg-white rounded-xl border border-gray-100 p-4 flex items-center gap-3">
            <div className={`w-10 h-10 rounded-lg ${card.bg} ${card.color} flex items-center justify-center flex-shrink-0`}>
              {card.icon}
            </div>
            <div>
              <p className={`text-2xl font-bold ${card.color}`}>{card.value}</p>
              <p className="text-xs text-gray-500">{card.label}</p>
            </div>
          </div>
        ))}
      </div>

      {/* Filters + Search */}
      <div className="bg-white rounded-xl border border-gray-100 p-4">
        <div className="flex flex-wrap gap-3 items-center">
          <div className="relative flex-1 min-w-[200px]">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              type="text"
              placeholder="Search by name, reference, account..."
              value={search}
              onChange={e => setSearch(e.target.value)}
              className="w-full pl-9 pr-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>

          <div className="flex items-center gap-2">
            <Filter size={14} className="text-gray-400" />
            <select
              value={statusFilter}
              onChange={e => setStatusFilter(e.target.value)}
              className="text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-brand-500"
            >
              <option value="ALL">All Statuses</option>
              {['QUEUED','DISPATCHED','EN_ROUTE','ON_SITE','COMPLETED','FAILED','CANCELLED','ESCALATED','SOS'].map(s => (
                <option key={s} value={s}>{s.replace(/_/g, ' ')}</option>
              ))}
            </select>

            <select
              value={alertFilter}
              onChange={e => setAlertFilter(e.target.value)}
              className="text-sm border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-brand-500"
            >
              <option value="ALL">All Alert Levels</option>
              {['CRITICAL','HIGH','MEDIUM','LOW','INFO'].map(a => (
                <option key={a} value={a}>{a}</option>
              ))}
            </select>
          </div>
        </div>
      </div>

      {/* Jobs Table */}
      <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
        <div className="px-4 py-3 border-b border-gray-100 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-gray-700">
            {filtered.length} job{filtered.length !== 1 ? 's' : ''}
          </h2>
          <div className="flex items-center gap-1.5">
            <div className="w-2 h-2 bg-green-400 rounded-full animate-pulse" />
            <span className="text-xs text-gray-400">Live — refreshes every 30s</span>
          </div>
        </div>

        {jobsLoading ? (
          <div className="flex items-center justify-center py-16">
            <div className="w-6 h-6 border-2 border-brand-500 border-t-transparent rounded-full animate-spin" />
          </div>
        ) : filtered.length === 0 ? (
          <div className="text-center py-16">
            <MapPin size={32} className="mx-auto text-gray-300 mb-3" />
            <p className="text-gray-500">No field jobs match your filters</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-gray-50 text-xs text-gray-500 uppercase tracking-wide">
                  <th className="px-4 py-3 text-left">Job / Customer</th>
                  <th className="px-4 py-3 text-left">Alert</th>
                  <th className="px-4 py-3 text-left">Status</th>
                  <th className="px-4 py-3 text-left">Officer</th>
                  <th className="px-4 py-3 text-left">Dispatched</th>
                  <th className="px-4 py-3 text-left">Variance</th>
                  <th className="px-4 py-3 text-left">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-50">
                {filtered.map(job => (
                  <JobRow
                    key={job.id}
                    job={job}
                    onAssign={setAssigningJob}
                    onViewMap={handleViewMap}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Assign Modal */}
      {assigningJob && (
        <AssignModal
          job={assigningJob}
          officers={officers}
          onAssign={(jobId, officerId) => assignMutation.mutate({ jobId, officerId })}
          onClose={() => setAssigningJob(null)}
        />
      )}
    </div>
  )
}
