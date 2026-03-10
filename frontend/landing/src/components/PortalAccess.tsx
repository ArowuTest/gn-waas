import { ExternalLink, Shield, BarChart3, Building2, Smartphone } from 'lucide-react'

const PORTALS = [
  {
    name: 'Admin Portal',
    role: 'System Admin / Super Admin',
    description: 'Full system oversight — anomaly flags, audit events, field jobs, NRW analysis, reports, user management and district settings.',
    icon: Shield,
    color: 'green',
    url: 'https://college-despite-tenant-lower.trycloudflare.com',
    devLogin: 'Super Admin',
    badge: 'LIVE',
  },
  {
    name: 'GWL Portal',
    role: 'GWL Manager / Supervisor',
    description: 'Ghana Water Limited case management — review billing variances, assign field officers, approve reclassifications and credit requests.',
    icon: BarChart3,
    color: 'blue',
    url: 'https://salary-specialist-bob-gonna.trycloudflare.com',
    devLogin: 'GWL Manager',
    badge: 'LIVE',
  },
  {
    name: 'Authority Portal',
    role: 'GRA Officer / MOF Auditor',
    description: 'Regulatory oversight — district NRW dashboard, anomaly review, field job monitoring and GRA compliance reporting.',
    icon: Building2,
    color: 'purple',
    url: 'https://celebrate-tension-shuttle-density.trycloudflare.com',
    devLogin: 'GRA Officer',
    badge: 'LIVE',
  },
  {
    name: 'Mobile App',
    role: 'Field Officer',
    description: 'Flutter-based offline-first mobile app for GPS-locked meter readings, photo evidence capture and field job completion.',
    icon: Smartphone,
    color: 'orange',
    url: 'https://github.com/ArowuTest/gn-waas',
    devLogin: null,
    badge: 'Flutter',
  },
]

const colorMap: Record<string, { bg: string; border: string; icon: string; btn: string; badge: string }> = {
  green:  { bg: 'bg-green-50',  border: 'border-green-200',  icon: 'bg-green-800 text-white',       btn: 'bg-green-800 hover:bg-green-900 text-white',   badge: 'bg-green-100 text-green-800' },
  blue:   { bg: 'bg-blue-50',   border: 'border-blue-200',   icon: 'bg-blue-700 text-white',         btn: 'bg-blue-700 hover:bg-blue-800 text-white',     badge: 'bg-blue-100 text-blue-800' },
  purple: { bg: 'bg-purple-50', border: 'border-purple-200', icon: 'bg-purple-700 text-white',       btn: 'bg-purple-700 hover:bg-purple-800 text-white', badge: 'bg-purple-100 text-purple-800' },
  orange: { bg: 'bg-orange-50', border: 'border-orange-200', icon: 'bg-orange-600 text-white',       btn: 'bg-orange-600 hover:bg-orange-700 text-white', badge: 'bg-orange-100 text-orange-800' },
}

export default function PortalAccess() {
  return (
    <section id="portals" className="py-20 bg-gray-50">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        {/* Header */}
        <div className="text-center mb-14">
          <span className="inline-block bg-green-100 text-green-800 text-xs font-semibold px-3 py-1 rounded-full uppercase tracking-wide mb-4">
            Live Demo
          </span>
          <h2 className="text-3xl sm:text-4xl font-bold text-gray-900 mb-4">
            Access the Live Portals
          </h2>
          <p className="text-lg text-gray-600 max-w-2xl mx-auto">
            All four portals are deployed and running. Click any portal to open it — use the
            <span className="font-semibold text-green-800"> Dev Login</span> buttons inside for instant access.
          </p>
        </div>

        {/* Portal Cards */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-10">
          {PORTALS.map((portal) => {
            const c = colorMap[portal.color]
            const Icon = portal.icon
            return (
              <div
                key={portal.name}
                className={`rounded-2xl border-2 ${c.border} ${c.bg} p-6 flex flex-col gap-4 shadow-sm hover:shadow-md transition-shadow`}
              >
                {/* Top row */}
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-3">
                    <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${c.icon}`}>
                      <Icon className="w-5 h-5" />
                    </div>
                    <div>
                      <h3 className="font-bold text-gray-900 text-lg leading-tight">{portal.name}</h3>
                      <p className="text-xs text-gray-500 mt-0.5">{portal.role}</p>
                    </div>
                  </div>
                  <span className={`text-xs font-bold px-2 py-1 rounded-full ${c.badge}`}>
                    {portal.badge}
                  </span>
                </div>

                {/* Description */}
                <p className="text-sm text-gray-600 leading-relaxed flex-1">{portal.description}</p>

                {/* Dev login hint */}
                {portal.devLogin && (
                  <div className="flex items-center gap-2 bg-white/70 rounded-lg px-3 py-2 border border-gray-200">
                    <span className="text-xs text-gray-500">Dev login:</span>
                    <span className="text-xs font-semibold text-gray-800">Click &ldquo;{portal.devLogin}&rdquo; button on login page</span>
                  </div>
                )}

                {/* CTA */}
                <a
                  href={portal.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={`inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl text-sm font-semibold transition-colors ${c.btn}`}
                >
                  {portal.devLogin ? 'Open Portal' : 'View on GitHub'}
                  <ExternalLink className="w-4 h-4" />
                </a>
              </div>
            )
          })}
        </div>

        {/* API info bar */}
        <div className="bg-gray-900 rounded-2xl px-6 py-4 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
          <div>
            <p className="text-white font-semibold text-sm">API Gateway</p>
            <p className="text-gray-400 text-xs mt-0.5">REST API · Dev Mode · PostgreSQL + Redis · RLS enforced</p>
          </div>
          <a
            href="https://assisted-order-throat-procedures.trycloudflare.com/health"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 bg-green-600 hover:bg-green-500 text-white text-xs font-semibold px-4 py-2 rounded-lg transition-colors whitespace-nowrap"
          >
            <span className="w-2 h-2 bg-green-300 rounded-full animate-pulse" />
            Health Check
            <ExternalLink className="w-3 h-3" />
          </a>
        </div>
      </div>
    </section>
  )
}
