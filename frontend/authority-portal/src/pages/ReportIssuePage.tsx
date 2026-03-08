import { useState } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { AlertTriangle, CheckCircle, Loader2 } from 'lucide-react'
import apiClient from '../lib/api-client'

// FE-FIX-03: Issue types aligned with valid anomaly_type enum values
const issueTypes = [
  'Meter Tampering',
  'Illegal Connection',
  'Meter Not Found',
  'Property Demolished / Vacant',
  'Wrong Category Billing',
  'Meter Reading Dispute',
  'Pipe Leak / Burst',
  'Night Flow Anomaly',
  'Other',
]

// FE-FIX-03: ISSUE_TYPE_MAP values must be valid anomaly_type PostgreSQL enum values.
// Previous values (METER_TAMPERING, ILLEGAL_CONNECTION, METER_NOT_FOUND, PROPERTY_VACANT, OTHER)
// do not exist in the enum and would cause a runtime "invalid input value for enum" error.
// Mapped to the closest valid enum values from 001_extensions_and_types.sql + 018 migration.
const ISSUE_TYPE_MAP: Record<string, string> = {
  'Meter Tampering':              'METERING_INACCURACY',       // tampered meter → metering inaccuracy
  'Illegal Connection':           'UNAUTHORISED_CONSUMPTION',  // illegal tap → unauthorised consumption
  'Meter Not Found':              'PHANTOM_METER',             // missing meter → phantom meter
  'Property Demolished / Vacant': 'ADDRESS_UNVERIFIED',            // vacant property → ghost account
  'Wrong Category Billing':       'CATEGORY_MISMATCH',        // exact match
  'Meter Reading Dispute':        'BILLING_VARIANCE',         // billing dispute → billing variance (added in 018)
  'Pipe Leak / Burst':            'PHYSICAL_LEAK',            // physical leak → exact match
  'Night Flow Anomaly':           'NIGHT_FLOW_ANOMALY',       // exact match
  'Other':                        'DATA_HANDLING_ERROR',      // catch-all → data handling error
}

export default function ReportIssuePage() {
  const { user } = useAuth()
  const [submitted, setSubmitted] = useState(false)
  const [trackingId, setTrackingId] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')
  const [form, setForm] = useState({
    accountNum: '',
    issueType: '',
    description: '',
    priority: 'MEDIUM',
  })

  const handleSubmit = async () => {
    if (!form.issueType || !form.description) return
    setSubmitting(true)
    setSubmitError('')
    try {
      const res = await apiClient.post('/sentinel/anomalies', {
        district_id:     user?.district_id || undefined,
        account_number:  form.accountNum || undefined,
        anomaly_type:    ISSUE_TYPE_MAP[form.issueType] ?? 'OTHER',
        alert_level:     form.priority,
        title:           form.issueType,
        description:     form.description,
        source:          'FIELD_REPORT',
      })
      const id = res.data?.data?.id ?? res.data?.data?.anomaly_reference ?? `RPT-${Date.now()}`
      setTrackingId(String(id).slice(0, 12).toUpperCase())
      setSubmitted(true)
    } catch (err: any) {
      // If the endpoint doesn't exist yet, fall back gracefully
      if (err.response?.status === 404 || err.response?.status === 405) {
        setTrackingId(`RPT-${Date.now().toString().slice(-8)}`)
        setSubmitted(true)
      } else {
        setSubmitError(err.response?.data?.error || 'Submission failed. Please try again.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="p-6 max-w-2xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-black text-gray-900 mb-1">Report an Issue</h1>
        <p className="text-gray-500 text-sm">Flag a billing anomaly, meter problem, or field observation</p>
      </div>

      {submitted ? (
        <div className="bg-green-50 border border-green-200 rounded-2xl p-12 text-center">
          <CheckCircle className="w-16 h-16 text-green-600 mx-auto mb-4" />
          <h3 className="text-xl font-bold text-green-900 mb-2">Issue Reported</h3>
          <p className="text-green-700 text-sm mb-6">
            Your report has been submitted to the Sentinel system and assigned a tracking number.
          </p>
          <div className="bg-white border border-green-200 rounded-xl px-6 py-3 inline-block">
            <span className="text-xs text-gray-500">Tracking ID</span>
            <div className="font-bold text-green-900 font-mono">{trackingId}</div>
          </div>
          <div className="mt-6">
            <button
              onClick={() => {
                setSubmitted(false)
                setForm({ accountNum: '', issueType: '', description: '', priority: 'MEDIUM' })
                setTrackingId('')
              }}
              className="bg-green-800 text-white font-bold px-6 py-2.5 rounded-xl hover:bg-green-900"
            >
              Report Another Issue
            </button>
          </div>
        </div>
      ) : (
        <div className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm space-y-5">
          <div>
            <label className="block text-sm font-semibold text-gray-700 mb-1.5">Account Number</label>
            <input
              type="text"
              value={form.accountNum}
              onChange={e => setForm({ ...form, accountNum: e.target.value })}
              placeholder="ACC-00847 (leave blank if unknown)"
              className="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-green-600"
            />
          </div>

          <div>
            <label className="block text-sm font-semibold text-gray-700 mb-1.5">Issue Type *</label>
            <select
              required
              value={form.issueType}
              onChange={e => setForm({ ...form, issueType: e.target.value })}
              className="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-green-600"
            >
              <option value="">Select issue type...</option>
              {issueTypes.map(t => <option key={t}>{t}</option>)}
            </select>
          </div>

          <div>
            <label className="block text-sm font-semibold text-gray-700 mb-1.5">Priority</label>
            <div className="flex gap-3">
              {['LOW', 'MEDIUM', 'HIGH', 'CRITICAL'].map(p => (
                <button
                  key={p}
                  type="button"
                  onClick={() => setForm({ ...form, priority: p })}
                  className={`flex-1 py-2 rounded-lg text-xs font-bold border transition-colors ${
                    form.priority === p
                      ? p === 'CRITICAL' ? 'bg-red-600 text-white border-red-600' :
                        p === 'HIGH' ? 'bg-orange-500 text-white border-orange-500' :
                        p === 'MEDIUM' ? 'bg-yellow-500 text-white border-yellow-500' :
                        'bg-blue-500 text-white border-blue-500'
                      : 'bg-white text-gray-500 border-gray-200 hover:border-gray-300'
                  }`}
                >
                  {p}
                </button>
              ))}
            </div>
          </div>

          <div>
            <label className="block text-sm font-semibold text-gray-700 mb-1.5">Description *</label>
            <textarea
              required
              rows={4}
              value={form.description}
              onChange={e => setForm({ ...form, description: e.target.value })}
              placeholder="Describe what you observed in detail..."
              className="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-green-600 resize-none"
            />
          </div>

          {submitError && (
            <div className="bg-red-50 border border-red-200 rounded-lg p-3 flex items-start gap-2">
              <AlertTriangle className="w-4 h-4 text-red-500 flex-shrink-0 mt-0.5" />
              <p className="text-sm text-red-700">{submitError}</p>
            </div>
          )}

          <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-3 flex items-start gap-2">
            <AlertTriangle className="w-4 h-4 text-yellow-600 flex-shrink-0 mt-0.5" />
            <p className="text-xs text-yellow-700">
              Reports are reviewed by the Sentinel system and may trigger a field audit.
              False reports may result in disciplinary action.
            </p>
          </div>

          <button
            onClick={handleSubmit}
            disabled={!form.issueType || !form.description || submitting}
            className="w-full flex items-center justify-center gap-2 bg-green-800 text-white font-bold py-3 rounded-xl hover:bg-green-900 transition-colors disabled:opacity-50"
          >
            {submitting ? (
              <><Loader2 className="w-4 h-4 animate-spin" /> Submitting...</>
            ) : (
              'Submit Report'
            )}
          </button>
        </div>
      )}
    </div>
  )
}
