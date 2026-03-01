import { CheckCircle } from 'lucide-react'

export default function BusinessModel() {
  return (
    <section id="business-model" className="py-24 bg-green-900 text-white">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-16">
          <div className="inline-block bg-yellow-400/20 border border-yellow-400/40 text-yellow-300 text-sm font-semibold px-4 py-1.5 rounded-full mb-4">
            Business Model
          </div>
          <h2 className="text-3xl sm:text-4xl font-black mb-4">
            Aligned Incentives. Sovereign Ownership.
          </h2>
          <p className="text-green-200 text-lg max-w-2xl mx-auto">
            GN-WAAS is structured so that GWL and GRA only pay when revenue is recovered.
            The system pays for itself.
          </p>
        </div>

        <div className="grid lg:grid-cols-3 gap-8 mb-16">
          {[
            {
              title: 'Deployment Fee',
              amount: 'GHS 2.5M',
              period: 'One-time',
              desc: 'Full system deployment, data migration, staff training, and 90-day pilot support.',
              items: ['Full source code ownership', 'NITA-certified deployment', '90-day pilot (Accra West / Tema)', 'Staff training & documentation'],
            },
            {
              title: 'Monthly Retainer',
              amount: 'GHS 150K',
              period: 'Per month',
              desc: 'Ongoing system maintenance, updates, GRA API compliance, and technical support.',
              items: ['System monitoring & updates', 'GRA API compliance maintenance', 'Sentinel algorithm tuning', 'Priority support SLA'],
              highlight: true,
            },
            {
              title: 'Success Fee',
              amount: '3%',
              period: 'Per recovery',
              desc: 'A percentage of every confirmed revenue recovery. GN-WAAS earns when GWL earns.',
              items: ['Only on confirmed recoveries', 'GRA-signed audit trail required', 'Transparent calculation', 'Monthly settlement'],
            },
          ].map(tier => (
            <div key={tier.title} className={`rounded-2xl p-8 ${
              tier.highlight
                ? 'bg-yellow-400 text-green-900'
                : 'bg-white/10 border border-white/20'
            }`}>
              <div className={`text-sm font-semibold mb-2 ${tier.highlight ? 'text-green-800' : 'text-green-300'}`}>
                {tier.period}
              </div>
              <div className={`text-4xl font-black mb-1 ${tier.highlight ? 'text-green-900' : 'text-yellow-400'}`}>
                {tier.amount}
              </div>
              <div className={`text-lg font-bold mb-4 ${tier.highlight ? 'text-green-800' : 'text-white'}`}>
                {tier.title}
              </div>
              <p className={`text-sm leading-relaxed mb-6 ${tier.highlight ? 'text-green-800' : 'text-green-200'}`}>
                {tier.desc}
              </p>
              <ul className="space-y-2">
                {tier.items.map(item => (
                  <li key={item} className={`flex items-center gap-2 text-sm ${tier.highlight ? 'text-green-800' : 'text-green-200'}`}>
                    <CheckCircle className={`w-4 h-4 flex-shrink-0 ${tier.highlight ? 'text-green-700' : 'text-green-400'}`} />
                    {item}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        {/* ROI callout */}
        <div className="bg-white/10 border border-white/20 rounded-2xl p-8 text-center">
          <div className="text-yellow-400 text-sm font-semibold mb-2">Return on Investment</div>
          <div className="text-4xl font-black mb-2">Break-even in Month 3</div>
          <p className="text-green-200 max-w-xl mx-auto">
            At GHS 120M annual commercial loss, recovering just 2% in the first 90-day pilot
            generates GHS 2.4M — covering the full deployment fee.
          </p>
        </div>
      </div>
    </section>
  )
}
