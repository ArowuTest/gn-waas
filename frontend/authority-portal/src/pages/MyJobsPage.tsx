import { MapPin, Clock, CheckCircle, AlertTriangle, Camera } from 'lucide-react'

const mockJobs = [
  { id: 'JOB-001', account: 'ACC-00847', customer: 'Kwame Asante', address: '14 Ring Road, Accra', type: 'Shadow Bill Variance', status: 'DISPATCHED', scheduledAt: '09:00', priority: 'HIGH' },
  { id: 'JOB-002', account: 'ACC-01203', customer: 'Tema Cold Store Ltd', address: 'Industrial Area, Tema', type: 'Category Mismatch', status: 'QUEUED', scheduledAt: '11:30', priority: 'MEDIUM' },
  { id: 'JOB-003', account: 'ACC-00512', customer: 'Ama Boateng', address: '7 Cantonments Rd', type: 'Phantom Meter', status: 'COMPLETED', scheduledAt: '08:00', priority: 'CRITICAL' },
]

const statusColor: Record<string, string> = {
  DISPATCHED: 'bg-blue-100 text-blue-700',
  QUEUED: 'bg-gray-100 text-gray-600',
  COMPLETED: 'bg-green-100 text-green-700',
  ON_SITE: 'bg-yellow-100 text-yellow-700',
  FAILED: 'bg-red-100 text-red-700',
}

export default function MyJobsPage() {
  return (
    <div className="p-6 max-w-3xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-black text-gray-900 mb-1">My Field Jobs</h1>
        <p className="text-gray-500 text-sm">Today's assigned audit jobs — {new Date().toLocaleDateString('en-GH', { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' })}</p>
      </div>

      {/* Summary */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        {[
          { label: 'Total Today', value: 3, icon: Clock, color: 'blue' },
          { label: 'Completed', value: 1, icon: CheckCircle, color: 'green' },
          { label: 'Pending', value: 2, icon: AlertTriangle, color: 'yellow' },
        ].map(({ label, value, icon: Icon, color }) => (
          <div key={label} className="bg-white rounded-xl p-4 border border-gray-100 shadow-sm text-center">
            <Icon className={`w-5 h-5 mx-auto mb-2 ${color === 'blue' ? 'text-blue-600' : color === 'green' ? 'text-green-700' : 'text-yellow-600'}`} />
            <div className="text-2xl font-black text-gray-900">{value}</div>
            <div className="text-xs text-gray-500">{label}</div>
          </div>
        ))}
      </div>

      {/* Job list */}
      <div className="space-y-4">
        {mockJobs.map(job => (
          <div key={job.id} className={`bg-white rounded-xl border p-5 shadow-sm ${job.status === 'COMPLETED' ? 'opacity-60' : 'border-gray-100 hover:border-green-200'}`}>
            <div className="flex items-start justify-between mb-3">
              <div>
                <div className="flex items-center gap-2 mb-1">
                  <span className="font-bold text-gray-900">{job.customer}</span>
                  <span className="text-xs text-gray-400">{job.id}</span>
                </div>
                <div className="flex items-center gap-1 text-sm text-gray-500">
                  <MapPin className="w-3 h-3" />
                  {job.address}
                </div>
              </div>
              <span className={`text-xs font-bold px-3 py-1 rounded-full ${statusColor[job.status]}`}>
                {job.status}
              </span>
            </div>

            <div className="flex items-center gap-3 text-xs text-gray-500 mb-4">
              <span className="bg-gray-100 px-2 py-0.5 rounded">{job.account}</span>
              <span className="bg-gray-100 px-2 py-0.5 rounded">{job.type}</span>
              <span className={`px-2 py-0.5 rounded font-medium ${
                job.priority === 'CRITICAL' ? 'bg-red-100 text-red-700' :
                job.priority === 'HIGH' ? 'bg-orange-100 text-orange-700' :
                'bg-yellow-100 text-yellow-700'
              }`}>{job.priority}</span>
              <span>Scheduled: {job.scheduledAt}</span>
            </div>

            {job.status !== 'COMPLETED' && (
              <div className="flex gap-2">
                <button className="flex-1 flex items-center justify-center gap-2 bg-green-800 text-white text-sm font-semibold py-2 rounded-lg hover:bg-green-900">
                  <Camera className="w-4 h-4" />
                  Start Audit
                </button>
                <button className="px-4 py-2 border border-red-200 text-red-600 text-sm font-semibold rounded-lg hover:bg-red-50">
                  SOS
                </button>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
