import { useState } from 'react'
import { Search, User, MapPin, Droplets, AlertTriangle } from 'lucide-react'

const mockAccounts = [
  { id: '1', accountNumber: 'ACC-00847', name: 'Kwame Asante', address: '14 Ring Road, Accra', category: 'RESIDENTIAL', meter: 'MTR-4421', status: 'ANOMALY', lastReading: 45.2 },
  { id: '2', accountNumber: 'ACC-01203', name: 'Tema Cold Store Ltd', address: 'Industrial Area, Tema', category: 'COMMERCIAL', meter: 'MTR-8832', status: 'NORMAL', lastReading: 312.8 },
  { id: '3', accountNumber: 'ACC-00512', name: 'Ama Boateng', address: '7 Cantonments Rd', category: 'RESIDENTIAL', meter: 'MTR-2201', status: 'ANOMALY', lastReading: 8.1 },
  { id: '4', accountNumber: 'ACC-02891', name: 'Kofi Mensah', address: '22 Spintex Road', category: 'RESIDENTIAL', meter: null, status: 'GHOST', lastReading: null },
]

export default function AccountSearchPage() {
  const [query, setQuery] = useState('')
  const filtered = mockAccounts.filter(a =>
    a.accountNumber.toLowerCase().includes(query.toLowerCase()) ||
    a.name.toLowerCase().includes(query.toLowerCase()) ||
    a.address.toLowerCase().includes(query.toLowerCase())
  )

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-black text-gray-900 mb-1">Account Search</h1>
        <p className="text-gray-500 text-sm">Search by account number, customer name, or address</p>
      </div>

      {/* Search bar */}
      <div className="relative mb-6">
        <Search className="absolute left-4 top-3.5 w-4 h-4 text-gray-400" />
        <input
          type="text"
          value={query}
          onChange={e => setQuery(e.target.value)}
          placeholder="Search accounts..."
          className="w-full pl-11 pr-4 py-3 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-green-600 bg-white shadow-sm"
        />
      </div>

      {/* Results */}
      <div className="space-y-3">
        {filtered.map(account => (
          <div key={account.id} className="bg-white rounded-xl border border-gray-100 p-5 shadow-sm hover:border-green-200 transition-colors cursor-pointer">
            <div className="flex items-start justify-between">
              <div className="flex items-start gap-4">
                <div className={`w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0 ${
                  account.status === 'ANOMALY' ? 'bg-red-100' :
                  account.status === 'GHOST' ? 'bg-orange-100' : 'bg-green-100'
                }`}>
                  {account.status === 'ANOMALY' ? <AlertTriangle className="w-5 h-5 text-red-600" /> :
                   account.status === 'GHOST' ? <User className="w-5 h-5 text-orange-600" /> :
                   <Droplets className="w-5 h-5 text-green-700" />}
                </div>
                <div>
                  <div className="flex items-center gap-2 mb-1">
                    <span className="font-bold text-gray-900">{account.name}</span>
                    <span className="text-xs bg-gray-100 text-gray-600 px-2 py-0.5 rounded-full">{account.accountNumber}</span>
                  </div>
                  <div className="flex items-center gap-1 text-sm text-gray-500">
                    <MapPin className="w-3 h-3" />
                    {account.address}
                  </div>
                  <div className="flex items-center gap-3 mt-2 text-xs text-gray-500">
                    <span className="bg-gray-100 px-2 py-0.5 rounded">{account.category}</span>
                    {account.meter ? (
                      <span>Meter: {account.meter}</span>
                    ) : (
                      <span className="text-orange-600 font-medium">No meter registered</span>
                    )}
                    {account.lastReading !== null && (
                      <span>Last reading: {account.lastReading} m³</span>
                    )}
                  </div>
                </div>
              </div>
              <span className={`text-xs font-bold px-3 py-1 rounded-full ${
                account.status === 'ANOMALY' ? 'bg-red-100 text-red-700' :
                account.status === 'GHOST' ? 'bg-orange-100 text-orange-700' :
                'bg-green-100 text-green-700'
              }`}>
                {account.status}
              </span>
            </div>
          </div>
        ))}
        {filtered.length === 0 && (
          <div className="text-center py-12 text-gray-400">
            <Search className="w-10 h-10 mx-auto mb-3 opacity-30" />
            <p>No accounts found for "{query}"</p>
          </div>
        )}
      </div>
    </div>
  )
}
