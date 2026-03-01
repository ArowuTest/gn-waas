const steps = [
  { n: '1', title: 'Mirror GWL Database', desc: 'CDC replication creates a read-only replica of GWL billing records. No disruption to live systems.', icon: '🔄' },
  { n: '2', title: 'Shadow Bill Calculation', desc: 'Every account is re-billed using PURC 2026 official tariffs. The Tariff Engine runs in parallel.', icon: '🧮' },
  { n: '3', title: 'Sentinel Scan', desc: '5 fraud detection algorithms run in parallel per district. Anomalies are deduplicated and scored.', icon: '🔍' },
  { n: '4', title: 'Field Dispatch', desc: 'High-confidence anomalies trigger blind field audits. Officers receive jobs on the mobile app.', icon: '📱' },
  { n: '5', title: 'Evidence Capture', desc: 'GPS-locked meter photos are processed by OCR. Evidence is hashed and stored immutably.', icon: '📸' },
  { n: '6', title: 'GRA Compliance', desc: 'Confirmed losses are submitted to GRA VSDC API. QR-signed receipts lock the audit record.', icon: '🏛️' },
]

export default function HowItWorks() {
  return (
    <section id="how-it-works" className="py-24 bg-gray-50">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-16">
          <div className="inline-block bg-blue-100 text-blue-700 text-sm font-semibold px-4 py-1.5 rounded-full mb-4">
            How It Works
          </div>
          <h2 className="text-3xl sm:text-4xl font-black text-gray-900 mb-4">
            From Data to Recovery in 6 Steps
          </h2>
          <p className="text-lg text-gray-600 max-w-2xl mx-auto">
            GN-WAAS runs continuously in the background, scanning every district every 6 hours.
          </p>
        </div>

        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-6">
          {steps.map(step => (
            <div key={step.n} className="bg-white rounded-2xl p-6 shadow-sm border border-gray-100 card-hover relative overflow-hidden">
              <div className="absolute top-4 right-4 text-6xl font-black text-gray-50 select-none">{step.n}</div>
              <div className="text-3xl mb-4">{step.icon}</div>
              <h3 className="font-bold text-gray-900 mb-2">{step.title}</h3>
              <p className="text-sm text-gray-600 leading-relaxed">{step.desc}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
