import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ReferenceLine, ResponsiveContainer } from 'recharts'

const data = [
  { district: 'Accra West', nrw: 38.2, grade: 'B' },
  { district: 'Tema', nrw: 44.1, grade: 'C' },
  { district: 'Accra East', nrw: 51.6, grade: 'D' },
  { district: 'Kumasi', nrw: 48.3, grade: 'C' },
  { district: 'Takoradi', nrw: 55.2, grade: 'D' },
]

export default function NRWSummaryPage() {
  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-black text-gray-900 mb-1">NRW Summary</h1>
        <p className="text-gray-500 text-sm">IWA/AWWA Water Balance — District Performance</p>
      </div>

      <div className="bg-white rounded-2xl border border-gray-100 p-6 shadow-sm mb-6">
        <h2 className="font-bold text-gray-900 mb-4">NRW % by District</h2>
        <ResponsiveContainer width="100%" height={280}>
          <BarChart data={data} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
            <XAxis dataKey="district" tick={{ fontSize: 12 }} />
            <YAxis tick={{ fontSize: 12 }} unit="%" domain={[0, 70]} />
            <Tooltip />
            <ReferenceLine y={20} stroke="#16a34a" strokeDasharray="4 4" label={{ value: 'IWA Target 20%', fill: '#16a34a', fontSize: 11 }} />
            <ReferenceLine y={51.6} stroke="#dc2626" strokeDasharray="4 4" label={{ value: 'Ghana Avg 51.6%', fill: '#dc2626', fontSize: 11 }} />
            <Bar dataKey="nrw" fill="#2e7d32" radius={[4, 4, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      </div>

      <div className="bg-white rounded-2xl border border-gray-100 shadow-sm overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 border-b border-gray-100">
            <tr>
              {['District', 'NRW %', 'Data Grade', 'Status'].map(h => (
                <th key={h} className="text-left px-5 py-3 text-xs font-semibold text-gray-500 uppercase tracking-wide">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-50">
            {data.map(row => (
              <tr key={row.district} className="hover:bg-gray-50">
                <td className="px-5 py-3 font-semibold text-gray-900">{row.district}</td>
                <td className="px-5 py-3">
                  <span className={`font-bold ${row.nrw > 50 ? 'text-red-600' : row.nrw > 30 ? 'text-orange-600' : 'text-green-700'}`}>
                    {row.nrw}%
                  </span>
                </td>
                <td className="px-5 py-3">
                  <span className={`font-bold px-2 py-0.5 rounded text-xs ${
                    row.grade === 'A' ? 'bg-green-100 text-green-700' :
                    row.grade === 'B' ? 'bg-blue-100 text-blue-700' :
                    row.grade === 'C' ? 'bg-yellow-100 text-yellow-700' :
                    'bg-red-100 text-red-700'
                  }`}>{row.grade}</span>
                </td>
                <td className="px-5 py-3">
                  <span className={`text-xs font-medium ${row.nrw > 50 ? 'text-red-600' : row.nrw > 30 ? 'text-orange-600' : 'text-green-700'}`}>
                    {row.nrw > 50 ? '⚠ Above Ghana Average' : row.nrw > 30 ? '↗ Above IWA Target' : '✓ Near Target'}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
