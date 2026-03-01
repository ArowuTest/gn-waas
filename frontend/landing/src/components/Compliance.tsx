export default function Compliance() {
  const badges = [
    { name: 'PURC 2026', desc: 'Official tariff rates', color: 'blue' },
    { name: 'GRA VSDC', desc: 'VAT invoice signing', color: 'green' },
    { name: 'IWA/AWWA', desc: 'Water balance standard', color: 'teal' },
    { name: 'NITA Ghana', desc: 'Data centre certified', color: 'purple' },
    { name: 'ISO 27001', desc: 'Security framework', color: 'red' },
    { name: 'GDPR Aligned', desc: 'Data privacy controls', color: 'orange' },
  ]

  return (
    <section id="compliance" className="py-24 bg-gray-50">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-12">
          <div className="inline-block bg-purple-100 text-purple-700 text-sm font-semibold px-4 py-1.5 rounded-full mb-4">
            Compliance & Standards
          </div>
          <h2 className="text-3xl sm:text-4xl font-black text-gray-900 mb-4">
            Built on Ghana's Regulatory Framework
          </h2>
          <p className="text-gray-600 max-w-xl mx-auto">
            Every component of GN-WAAS is aligned with Ghana's regulatory requirements
            and international water industry standards.
          </p>
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-4">
          {badges.map(b => (
            <div key={b.name} className="bg-white rounded-xl p-4 text-center shadow-sm border border-gray-100 card-hover">
              <div className={`w-10 h-10 rounded-lg mx-auto mb-3 flex items-center justify-center text-white font-bold text-xs ${
                b.color === 'blue' ? 'bg-blue-600' :
                b.color === 'green' ? 'bg-green-700' :
                b.color === 'teal' ? 'bg-teal-600' :
                b.color === 'purple' ? 'bg-purple-600' :
                b.color === 'orange' ? 'bg-orange-500' : 'bg-red-600'
              }`}>
                {b.name.slice(0, 2)}
              </div>
              <div className="font-bold text-gray-900 text-sm">{b.name}</div>
              <div className="text-xs text-gray-500 mt-1">{b.desc}</div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
