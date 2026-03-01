import { Smartphone, Lock, BarChart3, Globe, Zap, ShieldCheck } from 'lucide-react'

const features = [
  {
    icon: Zap,
    title: 'Real-Time Sentinel',
    desc: 'Automated scans every 6 hours. Night-flow analysis runs 2–4 AM to distinguish leaks from theft.',
  },
  {
    icon: Smartphone,
    title: 'Mobile Field App',
    desc: 'Offline-first app for field officers. Camera, GPS evidence capture, background sync, and biometric auth.',
  },
  {
    icon: Lock,
    title: 'Immutable Audit Trail',
    desc: 'Every audit event is append-only. GRA QR-code signing locks records permanently.',
  },
  {
    icon: BarChart3,
    title: 'IWA/AWWA Reporting',
    desc: 'NRW reports aligned with international standards. AWWA Data Confidence Grade A–F per district.',
  },
  {
    icon: Globe,
    title: 'Ghana Data Sovereignty',
    desc: 'Fully self-hosted. Deployed on NITA-certified data centres. No foreign cloud dependency.',
  },
  {
    icon: ShieldCheck,
    title: 'GRA Compliance Built-In',
    desc: 'Direct VSDC API integration. Every confirmed loss generates a GRA-signed VAT invoice.',
  },
]

export default function Features() {
  return (
    <section id="features" className="py-24 bg-white">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-16">
          <div className="inline-block bg-green-100 text-green-800 text-sm font-semibold px-4 py-1.5 rounded-full mb-4">
            Features
          </div>
          <h2 className="text-3xl sm:text-4xl font-black text-gray-900 mb-4">
            Built for Ghana. Built to Last.
          </h2>
        </div>
        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-6">
          {features.map(({ icon: Icon, title, desc }) => (
            <div key={title} className="p-6 rounded-2xl border border-gray-100 hover:border-green-200 hover:bg-green-50 transition-all card-hover">
              <div className="w-10 h-10 bg-green-100 rounded-xl flex items-center justify-center mb-4">
                <Icon className="w-5 h-5 text-green-700" />
              </div>
              <h3 className="font-bold text-gray-900 mb-2">{title}</h3>
              <p className="text-sm text-gray-600 leading-relaxed">{desc}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
