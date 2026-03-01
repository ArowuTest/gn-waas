import { Database, Brain, Users } from 'lucide-react'

const layers = [
  {
    icon: Database,
    number: '01',
    title: 'Financial Truth',
    subtitle: 'Shadow Billing Engine',
    desc: 'Every GWL bill is recalculated using PURC 2026 official tariffs. The Sentinel module compares the shadow bill against what GWL actually charged — flagging variances above 15%.',
    points: ['PURC 2026 tiered tariff rates', '20% VAT calculation (GRA 2026)', 'Automated variance detection', 'GRA VSDC API integration'],
    color: 'blue',
  },
  {
    icon: Brain,
    number: '02',
    title: 'Physical Truth',
    subtitle: 'Fraud Detection Engine',
    desc: 'Five parallel detection algorithms run against every account in a district: phantom meters, ghost accounts, category fraud, district imbalance, and mathematical impossibilities.',
    points: ['Phantom meter detection', 'Ghost account GPS validation', 'Night-flow anomaly analysis', 'Category mismatch detection'],
    color: 'green',
  },
  {
    icon: Users,
    number: '03',
    title: 'Operational Truth',
    subtitle: 'Field Audit Workflow',
    desc: 'Confirmed anomalies trigger blind field audits. Officers use the mobile app to capture GPS-locked meter photos. OCR reads the meter. Evidence is immutably stored.',
    points: ['GPS-locked evidence capture', 'Tesseract OCR meter reading', 'Blind audit workflow', 'Immutable audit trail'],
    color: 'yellow',
  },
]

export default function Solution() {
  return (
    <section id="solution" className="py-24 bg-white">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-16">
          <div className="inline-block bg-green-100 text-green-800 text-sm font-semibold px-4 py-1.5 rounded-full mb-4">
            The Solution
          </div>
          <h2 className="text-3xl sm:text-4xl font-black text-gray-900 mb-4">
            A Three-Layer Audit Engine
          </h2>
          <p className="text-lg text-gray-600 max-w-2xl mx-auto">
            GN-WAAS operates as a sovereign audit layer on top of GWL's existing systems.
            No disruption. No replacement. Pure oversight.
          </p>
        </div>

        <div className="grid lg:grid-cols-3 gap-8">
          {layers.map(({ icon: Icon, number, title, subtitle, desc, points, color }) => (
            <div key={title} className={`rounded-2xl p-8 border-2 card-hover ${
              color === 'blue' ? 'border-blue-100 bg-blue-50' :
              color === 'green' ? 'border-green-100 bg-green-50' :
              'border-yellow-100 bg-yellow-50'
            }`}>
              <div className="flex items-start gap-4 mb-6">
                <div className={`w-12 h-12 rounded-xl flex items-center justify-center flex-shrink-0 ${
                  color === 'blue' ? 'bg-blue-600' :
                  color === 'green' ? 'bg-green-700' : 'bg-yellow-500'
                }`}>
                  <Icon className="w-6 h-6 text-white" />
                </div>
                <div>
                  <div className={`text-xs font-bold mb-1 ${
                    color === 'blue' ? 'text-blue-500' :
                    color === 'green' ? 'text-green-600' : 'text-yellow-600'
                  }`}>LAYER {number}</div>
                  <h3 className="font-black text-gray-900 text-lg">{title}</h3>
                  <div className="text-sm text-gray-500">{subtitle}</div>
                </div>
              </div>

              <p className="text-gray-700 text-sm leading-relaxed mb-6">{desc}</p>

              <ul className="space-y-2">
                {points.map(point => (
                  <li key={point} className="flex items-center gap-2 text-sm text-gray-700">
                    <div className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${
                      color === 'blue' ? 'bg-blue-500' :
                      color === 'green' ? 'bg-green-600' : 'bg-yellow-500'
                    }`}></div>
                    {point}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
