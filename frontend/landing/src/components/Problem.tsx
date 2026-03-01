import { AlertTriangle, TrendingDown, DollarSign, Eye } from 'lucide-react'

const problems = [
  {
    icon: TrendingDown,
    stat: '51.6%',
    title: 'Non-Revenue Water',
    desc: 'Over half of all water produced by Ghana Water Limited never generates revenue — the highest NRW rate in West Africa.',
    color: 'red',
  },
  {
    icon: DollarSign,
    stat: 'GHS 120M+',
    title: 'Annual Revenue Loss',
    desc: 'Commercial losses from phantom meters, ghost accounts, category fraud, and systematic under-billing drain GWL every year.',
    color: 'orange',
  },
  {
    icon: AlertTriangle,
    stat: '20%+',
    title: 'Lost VAT',
    desc: 'Under-billed water means under-reported VAT. GRA cannot recover what was never invoiced correctly.',
    color: 'yellow',
  },
  {
    icon: Eye,
    stat: '0',
    title: 'Real-Time Oversight',
    desc: 'No system currently reconciles bulk meter production against household billing in real time. Fraud goes undetected for months.',
    color: 'blue',
  },
]

export default function Problem() {
  return (
    <section id="problem" className="py-24 bg-gray-50">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        {/* Header */}
        <div className="text-center mb-16">
          <div className="inline-block bg-red-100 text-red-700 text-sm font-semibold px-4 py-1.5 rounded-full mb-4">
            The Problem
          </div>
          <h2 className="text-3xl sm:text-4xl font-black text-gray-900 mb-4">
            Ghana's Water Revenue Crisis
          </h2>
          <p className="text-lg text-gray-600 max-w-2xl mx-auto">
            Ghana Water Limited produces water that never gets paid for. The gap between
            production and billing is not just a technical problem — it's a governance failure.
          </p>
        </div>

        {/* Problem cards */}
        <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-6 mb-16">
          {problems.map(({ icon: Icon, stat, title, desc, color }) => (
            <div key={title} className="bg-white rounded-2xl p-6 shadow-sm border border-gray-100 card-hover">
              <div className={`w-12 h-12 rounded-xl flex items-center justify-center mb-4 ${
                color === 'red' ? 'bg-red-100' :
                color === 'orange' ? 'bg-orange-100' :
                color === 'yellow' ? 'bg-yellow-100' : 'bg-blue-100'
              }`}>
                <Icon className={`w-6 h-6 ${
                  color === 'red' ? 'text-red-600' :
                  color === 'orange' ? 'text-orange-600' :
                  color === 'yellow' ? 'text-yellow-600' : 'text-blue-600'
                }`} />
              </div>
              <div className={`text-3xl font-black mb-1 ${
                color === 'red' ? 'text-red-600' :
                color === 'orange' ? 'text-orange-600' :
                color === 'yellow' ? 'text-yellow-600' : 'text-blue-600'
              }`}>{stat}</div>
              <h3 className="font-bold text-gray-900 mb-2">{title}</h3>
              <p className="text-sm text-gray-600 leading-relaxed">{desc}</p>
            </div>
          ))}
        </div>

        {/* IWA/AWWA Water Balance */}
        <div className="bg-green-900 rounded-2xl p-8 text-white">
          <div className="flex flex-col lg:flex-row items-start lg:items-center gap-6">
            <div className="flex-1">
              <div className="text-yellow-400 text-sm font-semibold mb-2">IWA/AWWA Water Balance Framework</div>
              <h3 className="text-2xl font-bold mb-3">Where Does the Water Go?</h3>
              <p className="text-green-200 text-sm leading-relaxed">
                The International Water Association framework breaks NRW into two components.
                GN-WAAS targets the <strong className="text-white">Commercial Loss</strong> component —
                the recoverable revenue that software can detect and recover.
              </p>
            </div>
            <div className="w-full lg:w-auto">
              <div className="space-y-2 min-w-64">
                {[
                  { label: 'System Input Volume', pct: 100, color: 'bg-blue-400', text: '100%' },
                  { label: 'Revenue Water (Billed)', pct: 48, color: 'bg-green-400', text: '48.4%' },
                  { label: 'Real Losses (Leaks)', pct: 28, color: 'bg-orange-400', text: '~28%' },
                  { label: 'Commercial Losses ← GN-WAAS', pct: 24, color: 'bg-red-400', text: '~24%' },
                ].map(item => (
                  <div key={item.label}>
                    <div className="flex justify-between text-xs text-green-300 mb-1">
                      <span>{item.label}</span>
                      <span className="font-bold text-white">{item.text}</span>
                    </div>
                    <div className="h-2 bg-white/10 rounded-full overflow-hidden">
                      <div className={`h-full ${item.color} rounded-full`} style={{ width: `${item.pct}%` }}></div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
