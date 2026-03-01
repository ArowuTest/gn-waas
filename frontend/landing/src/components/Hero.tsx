import { ArrowRight, Shield, TrendingDown, CheckCircle } from 'lucide-react'

export default function Hero() {
  return (
    <section className="hero-gradient min-h-screen flex items-center pt-16">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-24">
        <div className="grid lg:grid-cols-2 gap-16 items-center">
          {/* Left: Text */}
          <div>
            {/* Badge */}
            <div className="inline-flex items-center gap-2 bg-yellow-400/20 border border-yellow-400/40 rounded-full px-4 py-1.5 mb-6">
              <span className="w-2 h-2 bg-yellow-400 rounded-full animate-pulse"></span>
              <span className="text-yellow-300 text-sm font-medium">Pilot: Accra West & Tema Districts</span>
            </div>

            <h1 className="text-4xl sm:text-5xl lg:text-6xl font-black text-white leading-tight mb-6">
              Ghana's Water Revenue{' '}
              <span className="text-yellow-400">Audit Engine</span>
            </h1>

            <p className="text-lg text-green-100 leading-relaxed mb-8 max-w-xl">
              GN-WAAS is a sovereign audit layer that reconciles bulk meter data with billed water,
              detects commercial fraud, and recovers lost VAT — reducing Ghana's{' '}
              <strong className="text-yellow-300">51.6% Non-Revenue Water</strong> through software alone.
            </p>

            {/* Key stats */}
            <div className="grid grid-cols-3 gap-4 mb-10">
              {[
                { value: '51.6%', label: 'NRW Rate', sub: 'Ghana average' },
                { value: 'GHS 120M+', label: 'Annual Loss', sub: 'Recoverable' },
                { value: '3%', label: 'Success Fee', sub: 'Per recovery' },
              ].map(stat => (
                <div key={stat.label} className="bg-white/10 rounded-xl p-4 border border-white/20">
                  <div className="text-2xl font-black text-yellow-400">{stat.value}</div>
                  <div className="text-white text-sm font-semibold">{stat.label}</div>
                  <div className="text-green-300 text-xs">{stat.sub}</div>
                </div>
              ))}
            </div>

            {/* CTAs */}
            <div className="flex flex-wrap gap-4">
              <a
                href="#contact"
                className="inline-flex items-center gap-2 bg-yellow-400 text-green-900 font-bold px-6 py-3 rounded-xl hover:bg-yellow-300 transition-colors"
              >
                Request Demo <ArrowRight className="w-4 h-4" />
              </a>
              <a
                href="#how-it-works"
                className="inline-flex items-center gap-2 bg-white/10 text-white font-semibold px-6 py-3 rounded-xl border border-white/30 hover:bg-white/20 transition-colors"
              >
                See How It Works
              </a>
            </div>
          </div>

          {/* Right: Dashboard mockup */}
          <div className="hidden lg:block">
            <div className="bg-white/10 backdrop-blur-sm rounded-2xl border border-white/20 p-6 shadow-2xl">
              {/* Mock dashboard header */}
              <div className="flex items-center justify-between mb-6">
                <div>
                  <div className="text-white font-bold text-lg">Sentinel Dashboard</div>
                  <div className="text-green-300 text-sm">Accra West District — Live</div>
                </div>
                <div className="flex items-center gap-2 bg-green-500/20 border border-green-400/40 rounded-full px-3 py-1">
                  <span className="w-2 h-2 bg-green-400 rounded-full animate-pulse"></span>
                  <span className="text-green-300 text-xs font-medium">Scanning</span>
                </div>
              </div>

              {/* Mock anomaly cards */}
              <div className="space-y-3 mb-6">
                {[
                  { type: 'Shadow Bill Variance', account: 'ACC-00847', variance: '+₵2,340', level: 'CRITICAL', color: 'red' },
                  { type: 'Phantom Meter', account: 'ACC-01203', variance: '+₵890', level: 'HIGH', color: 'orange' },
                  { type: 'Category Mismatch', account: 'ACC-00512', variance: '+₵1,120', level: 'HIGH', color: 'orange' },
                  { type: 'Ghost Account', account: 'ACC-02891', variance: '+₵450', level: 'MEDIUM', color: 'yellow' },
                ].map(item => (
                  <div key={item.account} className="bg-white/10 rounded-lg p-3 flex items-center justify-between">
                    <div>
                      <div className="text-white text-sm font-semibold">{item.type}</div>
                      <div className="text-green-300 text-xs">{item.account}</div>
                    </div>
                    <div className="text-right">
                      <div className="text-yellow-400 text-sm font-bold">{item.variance}</div>
                      <div className={`text-xs font-medium ${
                        item.color === 'red' ? 'text-red-400' :
                        item.color === 'orange' ? 'text-orange-400' : 'text-yellow-400'
                      }`}>{item.level}</div>
                    </div>
                  </div>
                ))}
              </div>

              {/* Mock stats row */}
              <div className="grid grid-cols-3 gap-3">
                {[
                  { label: 'Open Flags', value: '47', icon: Shield },
                  { label: 'NRW Rate', value: '38.2%', icon: TrendingDown },
                  { label: 'Recovered', value: '₵84K', icon: CheckCircle },
                ].map(({ label, value, icon: Icon }) => (
                  <div key={label} className="bg-white/10 rounded-lg p-3 text-center">
                    <Icon className="w-4 h-4 text-yellow-400 mx-auto mb-1" />
                    <div className="text-white font-bold text-sm">{value}</div>
                    <div className="text-green-300 text-xs">{label}</div>
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
