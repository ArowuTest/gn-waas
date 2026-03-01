import { useState } from 'react'
import { Camera, MapPin, CheckCircle, AlertCircle } from 'lucide-react'

type Step = 'account' | 'photo' | 'reading' | 'confirm' | 'done'

export default function MeterReadingPage() {
  const [step, setStep] = useState<Step>('account')
  const [accountNum, setAccountNum] = useState('')
  const [reading, setReading] = useState('')
  const [notes, setNotes] = useState('')

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
            <input
              type="text"
              value={accountNum}
              onChange={e => setAccountNum(e.target.value)}
              placeholder="Enter account number (e.g. ACC-00847)"
              className="w-full border border-gray-200 rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-green-600 mb-4"
            />
            {accountNum && (
              <div className="bg-green-50 border border-green-200 rounded-lg p-4 mb-4">
                <div className="font-semibold text-green-900">Kwame Asante</div>
                <div className="text-sm text-green-700">14 Ring Road, Accra West</div>
                <div className="text-xs text-green-600 mt-1">Meter: MTR-4421 · Residential · Last reading: 45.2 m³</div>
              </div>
            )}
            <button
              onClick={() => setStep('photo')}
              disabled={!accountNum}
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
              Take a clear photo of the meter display. GPS location will be captured automatically.
            </p>
            <div className="border-2 border-dashed border-gray-200 rounded-xl p-12 text-center mb-4 bg-gray-50">
              <Camera className="w-12 h-12 text-gray-300 mx-auto mb-3" />
              <p className="text-sm text-gray-500 mb-3">Camera access required</p>
              <p className="text-xs text-gray-400">On mobile: tap to open camera</p>
              <p className="text-xs text-gray-400 mt-1">On desktop: upload meter photo</p>
            </div>
            <div className="flex items-center gap-2 text-xs text-green-700 bg-green-50 rounded-lg px-3 py-2 mb-4">
              <MapPin className="w-3 h-3" />
              <span>GPS: 5.6037° N, 0.1870° W · Accuracy: ±8m · ✓ Within fence</span>
            </div>
            <div className="flex gap-3">
              <button onClick={() => setStep('account')} className="flex-1 border border-gray-200 text-gray-600 font-semibold py-3 rounded-xl hover:bg-gray-50">
                Back
              </button>
              <button onClick={() => setStep('reading')} className="flex-1 bg-green-800 text-white font-bold py-3 rounded-xl hover:bg-green-900">
                Photo Captured →
              </button>
            </div>
          </div>
        )}

        {step === 'reading' && (
          <div>
            <h2 className="font-bold text-gray-900 mb-2">Enter Meter Reading</h2>
            <p className="text-sm text-gray-500 mb-4">OCR detected: <strong>47.8 m³</strong> (confidence: 94%). Verify or correct below.</p>
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

        {step === 'confirm' && (
          <div>
            <h2 className="font-bold text-gray-900 mb-4">Confirm Submission</h2>
            <div className="space-y-3 mb-6">
              {[
                { label: 'Account', value: accountNum },
                { label: 'Customer', value: 'Kwame Asante' },
                { label: 'Meter Reading', value: `${reading} m³` },
                { label: 'Previous Reading', value: '45.2 m³' },
                { label: 'Consumption', value: `${(parseFloat(reading || '0') - 45.2).toFixed(1)} m³` },
                { label: 'GPS Location', value: '5.6037° N, 0.1870° W ✓' },
                { label: 'Photo Hash', value: 'sha256:a3f8...c291' },
              ].map(({ label, value }) => (
                <div key={label} className="flex justify-between text-sm border-b border-gray-50 pb-2">
                  <span className="text-gray-500">{label}</span>
                  <span className="font-semibold text-gray-900">{value}</span>
                </div>
              ))}
            </div>
            <div className="flex gap-3">
              <button onClick={() => setStep('reading')} className="flex-1 border border-gray-200 text-gray-600 font-semibold py-3 rounded-xl hover:bg-gray-50">Back</button>
              <button onClick={() => setStep('done')} className="flex-1 bg-green-800 text-white font-bold py-3 rounded-xl hover:bg-green-900">Submit Reading</button>
            </div>
          </div>
        )}

        {step === 'done' && (
          <div className="text-center py-8">
            <CheckCircle className="w-16 h-16 text-green-600 mx-auto mb-4" />
            <h2 className="text-xl font-bold text-gray-900 mb-2">Reading Submitted</h2>
            <p className="text-gray-500 text-sm mb-6">
              Meter reading for {accountNum} has been recorded and submitted to the GN-WAAS platform.
            </p>
            <button
              onClick={() => { setStep('account'); setAccountNum(''); setReading(''); setNotes('') }}
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
