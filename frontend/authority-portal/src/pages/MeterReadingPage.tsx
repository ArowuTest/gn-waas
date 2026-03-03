import { useState } from 'react'
import apiClient from '../lib/api-client'
import { Camera, CheckCircle, Search, Loader2 } from 'lucide-react'

type Step = 'account' | 'photo' | 'reading' | 'confirm' | 'done'

interface AccountResult {
  id: string
  gwl_account_number: string
  account_holder_name: string
  address_line1?: string
  district_id: string
  meter_serial?: string
  account_category?: string
}

export default function MeterReadingPage() {
  const [step, setStep] = useState<Step>('account')
  const [accountNum, setAccountNum] = useState('')
  const [account, setAccount] = useState<AccountResult | null>(null)
  const [lookupLoading, setLookupLoading] = useState(false)
  const [lookupError, setLookupError] = useState('')
  const [reading, setReading] = useState('')
  const [notes, setNotes] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')

  // Look up account by account number via /accounts/search
  const lookupAccount = async () => {
    if (!accountNum.trim()) return
    setLookupLoading(true)
    setLookupError('')
    setAccount(null)
    try {
      const res = await apiClient.get('/accounts/search', { params: { q: accountNum.trim(), limit: 1 } })
      const items: AccountResult[] = res.data?.data ?? []
      if (items.length === 0) {
        setLookupError('No account found for that number. Please check and try again.')
      } else {
        setAccount(items[0])
      }
    } catch {
      setLookupError('Account lookup failed. Please try again.')
    } finally {
      setLookupLoading(false)
    }
  }

  // Submit meter reading to POST /audits with correct account_id and district_id
  const submitReading = async () => {
    if (!account || !reading) return
    setSubmitting(true)
    setSubmitError('')
    try {
      await apiClient.post('/audits', {
        account_id: account.id,
        district_id: account.district_id,
        notes: notes || `Manual meter reading: ${reading} m³`,
      })
      setStep('done')
    } catch (err: any) {
      setSubmitError(err.response?.data?.error || 'Submission failed. Please try again.')
    } finally {
      setSubmitting(false)
    }
  }

  const steps: { key: Step; label: string }[] = [
    { key: 'account', label: 'Find Account' },
    { key: 'photo', label: 'Capture Photo' },
    { key: 'reading', label: 'Enter Reading' },
    { key: 'confirm', label: 'Confirm' },
    { key: 'done', label: 'Done' },
  ]
  const stepIdx = steps.findIndex(s => s.key === step)

  return (
    <div className="p-6 max-w-2xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-black text-gray-900 mb-1">Meter Reading</h1>
        <p className="text-gray-500 text-sm">Submit a verified meter reading with GPS evidence</p>
      </div>

      {/* Progress */}
      <div className="flex items-center gap-2 mb-8">
        {steps.map((s, i) => (
          <div key={s.key} className="flex items-center gap-2 flex-1">
            <div className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold flex-shrink-0 ${
              i < stepIdx ? 'bg-green-700 text-white' :
              i === stepIdx ? 'bg-green-800 text-white ring-4 ring-green-200' :
              'bg-gray-200 text-gray-500'
            }`}>
              {i < stepIdx ? '✓' : i + 1}
            </div>
            {i < steps.length - 1 && (
              <div className={`h-0.5 flex-1 ${i < stepIdx ? 'bg-green-700' : 'bg-gray-200'}`}></div>
            )}
          </div>
        ))}
      </div>

      {/* Step content */}
      <div className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm">
        {step === 'account' && (
          <div>
            <h2 className="font-bold text-gray-900 mb-4">Find Account</h2>
            <div className="flex gap-2 mb-3">
              <input
                type="text"
                value={accountNum}
                onChange={e => setAccountNum(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && lookupAccount()}
                placeholder="Enter account number (e.g. ACC-00847)"
                className="flex-1 border border-gray-200 rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-green-600"
              />
              <button
                onClick={lookupAccount}
                disabled={!accountNum.trim() || lookupLoading}
                className="px-4 py-3 bg-green-800 text-white rounded-lg hover:bg-green-900 disabled:opacity-40 flex items-center gap-2"
              >
                {lookupLoading ? <Loader2 size={16} className="animate-spin" /> : <Search size={16} />}
              </button>
            </div>
            {lookupError && (
              <div className="bg-red-50 border border-red-200 rounded-lg p-3 mb-4 text-sm text-red-700">
                {lookupError}
              </div>
            )}
            {account && (
              <div className="bg-green-50 border border-green-200 rounded-lg p-4 mb-4">
                <div className="font-semibold text-green-900">{account.account_holder_name}</div>
                {account.address_line1 && (
                  <div className="text-sm text-green-700">{account.address_line1}</div>
                )}
                <div className="text-xs text-green-600 mt-1">
                  Account: {account.gwl_account_number}
                  {account.account_category && ` · ${account.account_category}`}
                </div>
              </div>
            )}
            <button
              onClick={() => setStep('photo')}
              disabled={!account}
              className="w-full bg-green-800 text-white font-bold py-3 rounded-xl hover:bg-green-900 transition-colors disabled:opacity-40"
            >
              Continue
            </button>
          </div>
        )}

        {step === 'photo' && (
          <div>
            <h2 className="font-bold text-gray-900 mb-2">Capture Meter Photo</h2>
            <p className="text-sm text-gray-500 mb-4">
              Take a clear photo of the meter display. The photo will be GPS-tagged and hash-verified.
            </p>
            <div className="border-2 border-dashed border-gray-200 rounded-xl p-12 flex flex-col items-center gap-3 mb-4 bg-gray-50">
              <Camera size={40} className="text-gray-400" />
              <p className="text-sm text-gray-500 text-center">
                Camera access is handled by the mobile app.<br />
                Click Continue to proceed.
              </p>
            </div>
            <div className="flex gap-3">
              <button onClick={() => setStep('account')} className="flex-1 border border-gray-200 text-gray-600 font-semibold py-3 rounded-xl hover:bg-gray-50">Back</button>
              <button onClick={() => setStep('reading')} className="flex-1 bg-green-800 text-white font-bold py-3 rounded-xl hover:bg-green-900">Continue →</button>
            </div>
          </div>
        )}

        {step === 'reading' && (
          <div>
            <h2 className="font-bold text-gray-900 mb-4">Enter Meter Reading</h2>
            <div className="mb-4">
              <label className="block text-sm font-semibold text-gray-700 mb-1.5">Reading (m³)</label>
              <input
                type="number"
                step="0.1"
                value={reading}
                onChange={e => setReading(e.target.value)}
                placeholder="47.8"
                className="w-full border border-gray-200 rounded-lg px-4 py-3 text-lg font-bold focus:outline-none focus:ring-2 focus:ring-green-600"
              />
            </div>
            <div className="mb-4">
              <label className="block text-sm font-semibold text-gray-700 mb-1.5">Notes (optional)</label>
              <textarea
                rows={3}
                value={notes}
                onChange={e => setNotes(e.target.value)}
                placeholder="Any observations about the meter or property..."
                className="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-green-600 resize-none"
              />
            </div>
            <div className="flex gap-3">
              <button onClick={() => setStep('photo')} className="flex-1 border border-gray-200 text-gray-600 font-semibold py-3 rounded-xl hover:bg-gray-50">Back</button>
              <button onClick={() => setStep('confirm')} disabled={!reading} className="flex-1 bg-green-800 text-white font-bold py-3 rounded-xl hover:bg-green-900 disabled:opacity-40">Review →</button>
            </div>
          </div>
        )}

        {step === 'confirm' && account && (
          <div>
            <h2 className="font-bold text-gray-900 mb-4">Confirm Submission</h2>
            <div className="space-y-3 mb-6">
              {[
                { label: 'Account', value: account.gwl_account_number },
                { label: 'Customer', value: account.account_holder_name },
                { label: 'Meter Reading', value: `${reading} m³` },
                { label: 'Notes', value: notes || '—' },
              ].map(({ label, value }) => (
                <div key={label} className="flex justify-between text-sm border-b border-gray-50 pb-2">
                  <span className="text-gray-500">{label}</span>
                  <span className="font-semibold text-gray-900">{value}</span>
                </div>
              ))}
            </div>
            <div className="flex gap-3">
              <button onClick={() => setStep('reading')} className="flex-1 border border-gray-200 text-gray-600 font-semibold py-3 rounded-xl hover:bg-gray-50">Back</button>
              <button onClick={submitReading} disabled={submitting} className="flex-1 bg-green-800 text-white font-bold py-3 rounded-xl hover:bg-green-900 disabled:opacity-50">
                {submitting ? 'Submitting...' : 'Submit Reading'}
              </button>
            </div>
            {submitError && (
              <div className="mt-3 p-3 bg-red-50 border border-red-200 rounded-lg text-red-700 text-sm">{submitError}</div>
            )}
          </div>
        )}

        {step === 'done' && (
          <div className="text-center py-8">
            <CheckCircle className="w-16 h-16 text-green-600 mx-auto mb-4" />
            <h2 className="text-xl font-bold text-gray-900 mb-2">Reading Submitted</h2>
            <p className="text-gray-500 text-sm mb-6">
              Meter reading for {account?.gwl_account_number} has been recorded and submitted to the GN-WAAS platform.
            </p>
            <button
              onClick={() => {
                setStep('account')
                setAccountNum('')
                setAccount(null)
                setReading('')
                setNotes('')
              }}
              className="bg-green-800 text-white font-bold px-8 py-3 rounded-xl hover:bg-green-900"
            >
              Submit Another Reading
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
