/**
 * GN-WAAS Authority Portal — Job Assignment Page
 *
 * Allows authority supervisors to view all field jobs, assign officers,
 * and monitor SOS alerts in real time.
 */

import { useState } from 'react'
import {
  MapPin, User, AlertTriangle, CheckCircle, Clock, RefreshCw,
  Loader2, UserCheck, Siren, Filter,
} from 'lucide-react'
import { useAllFieldJobs, useAssignOfficer, useFieldOfficersList } from '../hooks/useQueries'
import type { FieldJob } from '../types'

const STATUS_COLORS: Record<string, string> = {
  QUEUED:     'bg-gray-100 text-gray-600',
  DISPATCHED: 'bg-blue-100 text-blue-700',
  EN_ROUTE:   'bg-indigo-100 text-indigo-700',
  ON_SITE:    'bg-yellow-100 text-yellow-700',
  COMPLETED:  'bg-green-100 text-green-700',
  CANCELLED:  'bg-red-100 text-red-600',
  SOS:        'bg-red-600 text-white animate-pulse',
}

const PRIORITY_LABELS: Record<number, string> = {
  1: 'CRITICAL', 2: 'HIGH', 3: 'MEDIUM', 4: 'LOW',
}
const PRIORITY_COLORS: Record<number, string> = {
  1: 'text-red-600 font-bold', 2: 'text-orange-600 font-semibold',
  3: 'text-yellow-600', 4: 'text-gray-500',
}

const STATUS_OPTIONS = ['', 'QUEUED', 'DISPATCHED', 'EN_ROUTE', 'ON_SITE', 'COMPLETED', 'SOS']

function AssignModal({
  job,
  onClose,
}: {
  job: FieldJob
  onClose: () => void
}) {
  const [selectedOfficer, setSelectedOfficer] = useState('')
  const { data: officers = [], isLoading } = useFieldOfficersList(job.district_id)
  const assign = useAssignOfficer()

  const handleAssign = () => {
    if (!selectedOfficer) return
    assign.mutate(
      { jobId: job.id, officerId: selectedOfficer },
      { onSuccess: onClose },
    )
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-2xl shadow-2xl w-full max-w-md">
        <div className="p-6 border-b border-gray-100">
          <h2 className="text-lg font-bold text-gray-900">Assign Field Officer</h2>
          <p className="text-sm text-gray-500 mt-1">Job: {job.job_reference}</p>
        </div>
        <div className="p-6 space-y-4">
          {isLoading ? (
            <div className="flex items-center gap-2 text-gray-500">
              <Loader2 className="w-4 h-4 animate-spin" />
              <span>Loading officers...</span>
            </div>
          ) : officers.length === 0 ? (
            <p className="text-gray-500 text-sm">No field officers available in this district.</p>
          ) : (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                Select Officer
              </label>
              <select
                value={selectedOfficer}
                onChange={e => setSelectedOfficer(e.target.value)}
                className="w-full border border-gray-200 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-green-500"
              >
                <option value="">-- Choose an officer --</option>
                {officers.map(o => (
                  <option key={o.id} value={o.id}>
                    {o.full_name} ({o.email})
                  </option>
                ))}
              </select>
            </div>
          )}
        </div>
        <div className="p-6 border-t border-gray-100 flex gap-3 justify-end">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleAssign}
            disabled={!selectedOfficer || assign.isPending}
            className="px-4 py-2 bg-green-700 text-white text-sm rounded-lg hover:bg-green-800 disabled:opacity-50 flex items-center gap-2"
          >
            {assign.isPending && <Loader2 className="w-4 h-4 animate-spin" />}
            <UserCheck className="w-4 h-4" />
            Assign Officer
          </button>
        </div>
      </div>
    </div>
  )
}

function JobRow({ job, onAssign }: { job: FieldJob; onAssign: (job: FieldJob) => void }) {
  const isSOS = job.status === 'SOS'
  return (
    <tr className={`border-b border-gray-50 hover:bg-gray-50 transition-colors ${isSOS ? 'bg-red-50' : ''}`}>
      <td className="px-4 py-3">
        <div className="flex items-center gap-2">
          {isSOS && <Siren className="w-4 h-4 text-red-600 animate-pulse" />}
          <div>
            <p className="text-sm font-mono font-medium text-gray-900">{job.job_reference}</p>
            <p className="text-xs text-gray-400">{new Date(job.created_at).toLocaleDateString()}</p>
          </div>
        </div>
      </td>
      <td className="px-4 py-3">
        <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${STATUS_COLORS[job.status] ?? 'bg-gray-100 text-gray-600'}`}>
          {job.status}
        </span>
      </td>
      <td className="px-4 py-3">
        <span className={`text-xs font-medium ${PRIORITY_COLORS[job.priority] ?? 'text-gray-500'}`}>
          {PRIORITY_LABELS[job.priority] ?? 'UNKNOWN'}
        </span>
      </td>
      <td className="px-4 py-3">
        <div className="flex items-center gap-1 text-xs text-gray-600">
          <MapPin className="w-3 h-3" />
          <span>{job.target_gps_lat?.toFixed(4)}, {job.target_gps_lng?.toFixed(4)}</span>
        </div>
      </td>
      <td className="px-4 py-3">
        {job.assigned_officer_id ? (
          <div className="flex items-center gap-1 text-xs text-green-700">
            <User className="w-3 h-3" />
            <span>Assigned</span>
          </div>
        ) : (
          <span className="text-xs text-gray-400">Unassigned</span>
        )}
      </td>
      <td className="px-4 py-3">
        <button
          onClick={() => onAssign(job)}
          disabled={job.status === 'COMPLETED' || job.status === 'CANCELLED'}
          className="text-xs px-3 py-1 bg-green-700 text-white rounded-lg hover:bg-green-800 disabled:opacity-40 disabled:cursor-not-allowed flex items-center gap-1"
        >
          <UserCheck className="w-3 h-3" />
          Assign
        </button>
      </td>
    </tr>
  )
}

export default function JobAssignmentPage() {
  const [statusFilter, setStatusFilter] = useState('')
  const [assignTarget, setAssignTarget] = useState<FieldJob | null>(null)

  const { data: jobs = [], isLoading, isError, refetch, isFetching } = useAllFieldJobs(statusFilter || undefined)

  const sosJobs = jobs.filter(j => j.status === 'SOS')
  const activeJobs = jobs.filter(j => !['COMPLETED', 'CANCELLED'].includes(j.status))
  const completedJobs = jobs.filter(j => j.status === 'COMPLETED')

  return (
    <div className="p-6 max-w-7xl mx-auto space-y-6">
      {/* SOS Alert Banner */}
      {sosJobs.length > 0 && (
        <div className="bg-red-600 text-white rounded-xl p-4 flex items-center gap-3 animate-pulse">
          <Siren className="w-6 h-6 flex-shrink-0" />
          <div>
            <p className="font-bold text-lg">⚠ SOS ALERT — {sosJobs.length} officer{sosJobs.length > 1 ? 's' : ''} in distress</p>
            <p className="text-red-100 text-sm">
              {sosJobs.map(j => j.job_reference).join(', ')} — Dispatch security escort immediately
            </p>
          </div>
        </div>
      )}

      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Field Job Assignment</h1>
          <p className="text-gray-500 text-sm mt-1">Assign officers to field jobs and monitor progress</p>
        </div>
        <button
          onClick={() => refetch()}
          disabled={isFetching}
          className="flex items-center gap-2 px-4 py-2 text-sm text-gray-600 border border-gray-200 rounded-lg hover:bg-gray-50"
        >
          <RefreshCw className={`w-4 h-4 ${isFetching ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      {/* KPI Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[
          { label: 'Total Jobs', value: jobs.length, icon: Clock, color: 'text-gray-700' },
          { label: 'Active', value: activeJobs.length, icon: AlertTriangle, color: 'text-orange-600' },
          { label: 'SOS Alerts', value: sosJobs.length, icon: Siren, color: 'text-red-600' },
          { label: 'Completed', value: completedJobs.length, icon: CheckCircle, color: 'text-green-600' },
        ].map(({ label, value, icon: Icon, color }) => (
          <div key={label} className="bg-white rounded-xl border border-gray-100 p-4 shadow-sm">
            <div className="flex items-center gap-2 mb-1">
              <Icon className={`w-4 h-4 ${color}`} />
              <span className="text-xs text-gray-500">{label}</span>
            </div>
            <p className={`text-2xl font-bold ${color}`}>{value}</p>
          </div>
        ))}
      </div>

      {/* Filters */}
      <div className="bg-white rounded-xl border border-gray-100 p-4 shadow-sm flex items-center gap-4">
        <Filter className="w-4 h-4 text-gray-400" />
        <div className="flex items-center gap-2">
          <label className="text-sm text-gray-600">Status:</label>
          <select
            value={statusFilter}
            onChange={e => setStatusFilter(e.target.value)}
            className="border border-gray-200 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-green-500"
          >
            {STATUS_OPTIONS.map(s => (
              <option key={s} value={s}>{s || 'All Statuses'}</option>
            ))}
          </select>
        </div>
      </div>

      {/* Jobs Table */}
      <div className="bg-white rounded-xl border border-gray-100 shadow-sm overflow-hidden">
        {isLoading ? (
          <div className="flex items-center justify-center py-16">
            <Loader2 className="w-8 h-8 animate-spin text-green-700" />
          </div>
        ) : isError ? (
          <div className="flex items-center justify-center py-16 text-red-500">
            <AlertTriangle className="w-5 h-5 mr-2" />
            Failed to load field jobs
          </div>
        ) : jobs.length === 0 ? (
          <div className="flex items-center justify-center py-16 text-gray-400">
            <CheckCircle className="w-5 h-5 mr-2" />
            No field jobs found
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-gray-50 border-b border-gray-100">
                <tr>
                  {['Job Reference', 'Status', 'Priority', 'Location', 'Officer', 'Action'].map(h => (
                    <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {jobs.map(job => (
                  <JobRow key={job.id} job={job} onAssign={setAssignTarget} />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Assign Modal */}
      {assignTarget && (
        <AssignModal job={assignTarget} onClose={() => setAssignTarget(null)} />
      )}
    </div>
  )
}
