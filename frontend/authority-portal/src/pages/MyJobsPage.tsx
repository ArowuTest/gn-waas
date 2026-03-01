import { MapPin, Clock, CheckCircle, AlertTriangle, Camera, Loader2, RefreshCw } from 'lucide-react'
import { useMyJobs, useUpdateJobStatus, useTriggerSOS } from '../hooks/useQueries'
import type { FieldJob } from '../types'

const statusColor: Record<string, string> = {
  DISPATCHED: 'bg-blue-100 text-blue-700',
  ASSIGNED: 'bg-gray-100 text-gray-600',
  ARRIVED: 'bg-yellow-100 text-yellow-700',
  COMPLETED: 'bg-green-100 text-green-700',
  CANCELLED: 'bg-red-100 text-red-700',
}

const priorityColor: Record<number, string> = {
  1: 'bg-red-100 text-red-700',
  2: 'bg-orange-100 text-orange-700',
  3: 'bg-yellow-100 text-yellow-700',
  4: 'bg-gray-100 text-gray-600',
}

const priorityLabel: Record<number, string> = {
  1: 'CRITICAL',
  2: 'HIGH',
  3: 'MEDIUM',
  4: 'LOW',
}

function JobCard({ job }: { job: FieldJob }) {
  const updateStatus = useUpdateJobStatus()
  const triggerSOS = useTriggerSOS()

  const handleStart = () => {
    // Get GPS if available
    if (navigator.geolocation) {
      navigator.geolocation.getCurrentPosition(
        (pos) => {
          updateStatus.mutate({
            jobId: job.id,
            status: 'ARRIVED',
            gpsLat: pos.coords.latitude,
            gpsLng: pos.coords.longitude,
          })
        },
        () => updateStatus.mutate({ jobId: job.id, status: 'ARRIVED' })
      )
    } else {
      updateStatus.mutate({ jobId: job.id, status: 'ARRIVED' })
    }
  }

  const handleSOS = () => {
    if (navigator.geolocation) {
      navigator.geolocation.getCurrentPosition(
        (pos) => {
          triggerSOS.mutate({
            jobId: job.id,
            gpsLat: pos.coords.latitude,
            gpsLng: pos.coords.longitude,
          })
        },
        () => alert('Could not get GPS location for SOS. Please call emergency services directly.')
      )
    }
  }

  const isCompleted = job.status === 'COMPLETED' || job.status === 'CANCELLED'

  return (
    <div className={`bg-white rounded-xl border p-5 shadow-sm ${isCompleted ? 'opacity-60' : 'border-gray-100 hover:border-green-200'}`}>
      <div className="flex items-start justify-between mb-3">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <span className="font-bold text-gray-900">{job.job_reference}</span>
            {job.is_blind_audit && (
              <span className="text-xs bg-purple-100 text-purple-700 px-2 py-0.5 rounded-full font-medium">
                Blind Audit
              </span>
            )}
          </div>
          <div className="flex items-center gap-1 text-sm text-gray-500">
            <MapPin className="w-3 h-3" />
            {job.target_gps_lat.toFixed(4)}, {job.target_gps_lng.toFixed(4)}
          </div>
        </div>
        <span className={`text-xs font-bold px-3 py-1 rounded-full ${statusColor[job.status] ?? 'bg-gray-100 text-gray-600'}`}>
          {job.status}
        </span>
      </div>

      <div className="flex items-center gap-3 text-xs text-gray-500 mb-4 flex-wrap">
        <span className={`px-2 py-0.5 rounded font-medium ${priorityColor[job.priority] ?? 'bg-gray-100 text-gray-600'}`}>
          {priorityLabel[job.priority] ?? `P${job.priority}`}
        </span>
        {job.requires_security_escort && (
          <span className="bg-red-100 text-red-700 px-2 py-0.5 rounded font-medium">
            Security Escort Required
          </span>
        )}
        {job.dispatched_at && (
          <span>Dispatched: {new Date(job.dispatched_at).toLocaleTimeString('en-GH', { hour: '2-digit', minute: '2-digit' })}</span>
        )}
      </div>

      {!isCompleted && (
        <div className="flex gap-2">
          <button
            onClick={handleStart}
            disabled={updateStatus.isPending}
            className="flex-1 flex items-center justify-center gap-2 bg-green-800 text-white text-sm font-semibold py-2 rounded-lg hover:bg-green-900 disabled:opacity-50"
          >
            {updateStatus.isPending ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Camera className="w-4 h-4" />
            )}
            {job.status === 'ASSIGNED' ? 'Start Job' : 'Continue Audit'}
          </button>
          <button
            onClick={handleSOS}
            disabled={triggerSOS.isPending}
            className="px-4 py-2 border border-red-200 text-red-600 text-sm font-semibold rounded-lg hover:bg-red-50 disabled:opacity-50"
          >
            {triggerSOS.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : 'SOS'}
          </button>
        </div>
      )}
    </div>
  )
}

export default function MyJobsPage() {
  const { data: jobs, isLoading, isError, refetch, isFetching } = useMyJobs()

  const today = new Date().toLocaleDateString('en-GH', {
    weekday: 'long', year: 'numeric', month: 'long', day: 'numeric',
  })

  const total = jobs?.length ?? 0
  const completed = jobs?.filter(j => j.status === 'COMPLETED').length ?? 0
  const pending = total - completed

  if (isLoading) {
    return (
      <div className="p-6 flex items-center justify-center min-h-64">
        <Loader2 className="w-8 h-8 animate-spin text-green-700" />
      </div>
    )
  }

  if (isError) {
    return (
      <div className="p-6 max-w-3xl mx-auto">
        <div className="bg-red-50 border border-red-200 rounded-xl p-6 text-center">
          <AlertTriangle className="w-8 h-8 text-red-500 mx-auto mb-2" />
          <p className="text-red-700 font-semibold">Failed to load jobs</p>
          <button onClick={() => refetch()} className="mt-3 text-sm text-red-600 underline">
            Try again
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="p-6 max-w-3xl mx-auto">
      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-black text-gray-900 mb-1">My Field Jobs</h1>
          <p className="text-gray-500 text-sm">{today}</p>
        </div>
        <button
          onClick={() => refetch()}
          disabled={isFetching}
          className="p-2 rounded-lg border border-gray-200 hover:bg-gray-50 disabled:opacity-50"
          title="Refresh jobs"
        >
          <RefreshCw className={`w-4 h-4 text-gray-500 ${isFetching ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {/* Summary */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        {[
          { label: 'Total Today', value: total, icon: Clock, color: 'blue' },
          { label: 'Completed', value: completed, icon: CheckCircle, color: 'green' },
          { label: 'Pending', value: pending, icon: AlertTriangle, color: 'yellow' },
        ].map(({ label, value, icon: Icon, color }) => (
          <div key={label} className="bg-white rounded-xl p-4 border border-gray-100 shadow-sm text-center">
            <Icon className={`w-5 h-5 mx-auto mb-2 ${
              color === 'blue' ? 'text-blue-600' :
              color === 'green' ? 'text-green-700' :
              'text-yellow-600'
            }`} />
            <div className="text-2xl font-black text-gray-900">{value}</div>
            <div className="text-xs text-gray-500">{label}</div>
          </div>
        ))}
      </div>

      {/* Job list */}
      {jobs && jobs.length > 0 ? (
        <div className="space-y-4">
          {jobs.map(job => <JobCard key={job.id} job={job} />)}
        </div>
      ) : (
        <div className="bg-white rounded-xl border border-gray-100 p-12 text-center">
          <CheckCircle className="w-12 h-12 text-green-500 mx-auto mb-3" />
          <p className="text-gray-600 font-semibold">No jobs assigned today</p>
          <p className="text-gray-400 text-sm mt-1">Check back later or contact your supervisor</p>
        </div>
      )}
    </div>
  )
}
