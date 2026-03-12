import { useState, useCallback } from 'react'
import { Search, User, MapPin, Droplets, AlertTriangle, Loader2 } from 'lucide-react'
import { useAccountSearch } from '../hooks/useQueries'
import type { WaterAccount } from '../types'

function AccountCard({ account }: { account: WaterAccount }) {
  const hasAnomaly = account.is_phantom_flagged
  const statusLabel = hasAnomaly ? 'FLAGGED' : account.status

  return (
    <div className="bg-white rounded-xl border border-gray-100 p-5 shadow-sm hover:border-green-200 transition-colors cursor-pointer">
      <div className="flex items-start justify-between">
        <div className="flex items-start gap-4">
          <div className={`w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0 ${
            hasAnomaly ? 'bg-red-100' :
            account.status === 'INACTIVE' ? 'bg-orange-100' : 'bg-green-100'
          }`}>
            {hasAnomaly ? (
              <AlertTriangle className="w-5 h-5 text-red-600" />
            ) : account.status === 'INACTIVE' ? (
              <User className="w-5 h-5 text-orange-600" />
            ) : (
              <Droplets className="w-5 h-5 text-green-700" />
            )}
          </div>
          <div>
            <div className="flex items-center gap-2 mb-1">
              <span className="font-bold text-gray-900">{account.account_holder_name}</span>
              <span className="text-xs bg-gray-100 text-gray-600 px-2 py-0.5 rounded-full">
                {account.gwl_account_number}
              </span>
            </div>
            <div className="flex items-center gap-1 text-sm text-gray-500">
              <MapPin className="w-3 h-3" />
              {account.address_line1}
            </div>
            <div className="flex items-center gap-3 mt-2 text-xs text-gray-500 flex-wrap">
              <span className="bg-gray-100 px-2 py-0.5 rounded">{account.category}</span>
              {account.meter_number ? (
                <span>Meter: {account.meter_number}</span>
              ) : (
                <span className="text-orange-600 font-medium">No meter registered</span>
              )}
              {account.monthly_avg_consumption > 0 && (
                <span>Avg: {(account.monthly_avg_consumption ?? 0).toFixed(1)} m³/mo</span>
              )}
              {account.account_holder_tin && (
                <span>TIN: {account.account_holder_tin}</span>
              )}
            </div>
          </div>
        </div>
        <span className={`text-xs font-bold px-3 py-1 rounded-full flex-shrink-0 ${
          hasAnomaly ? 'bg-red-100 text-red-700' :
          account.status === 'INACTIVE' ? 'bg-orange-100 text-orange-700' :
          'bg-green-100 text-green-700'
        }`}>
          {statusLabel}
        </span>
      </div>
    </div>
  )
}

export default function AccountSearchPage() {
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')

  // Debounce search to avoid hammering the API on every keystroke
  const handleQueryChange = useCallback((value: string) => {
    setQuery(value)
    const timer = setTimeout(() => setDebouncedQuery(value), 400)
    return () => clearTimeout(timer)
  }, [])

  const { data, isLoading, isFetching, isError } = useAccountSearch(debouncedQuery)
  const accounts = data?.accounts ?? []
  const total = data?.total ?? 0

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-black text-gray-900 mb-1">Account Search</h1>
        <p className="text-gray-500 text-sm">Search by account number, customer name, address, or meter number</p>
      </div>

      {/* Search bar */}
      <div className="relative mb-6">
        <Search className="absolute left-4 top-3.5 w-4 h-4 text-gray-400" />
        <input
          type="text"
          value={query}
          onChange={e => handleQueryChange(e.target.value)}
          placeholder="Search accounts... (min. 2 characters)"
          className="w-full pl-11 pr-4 py-3 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-green-600 bg-white shadow-sm"
        />
        {isFetching && (
          <Loader2 className="absolute right-4 top-3.5 w-4 h-4 text-gray-400 animate-spin" />
        )}
      </div>

      {/* Results count */}
      {debouncedQuery.length >= 2 && !isLoading && (
        <p className="text-sm text-gray-500 mb-4">
          {total > 0 ? `${total} account${total !== 1 ? 's' : ''} found` : 'No accounts found'}
        </p>
      )}

      {/* Loading state */}
      {isLoading && debouncedQuery.length >= 2 && (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="w-8 h-8 animate-spin text-green-700" />
        </div>
      )}

      {/* Error state */}
      {isError && (
        <div className="bg-red-50 border border-red-200 rounded-xl p-6 text-center">
          <AlertTriangle className="w-8 h-8 text-red-500 mx-auto mb-2" />
          <p className="text-red-700 font-semibold">Search failed</p>
          <p className="text-red-500 text-sm mt-1">Please try again</p>
        </div>
      )}

      {/* Results */}
      {!isLoading && accounts.length > 0 && (
        <div className="space-y-3">
          {accounts.map(account => (
            <AccountCard key={account.id} account={account} />
          ))}
        </div>
      )}

      {/* Empty state */}
      {!isLoading && !isError && debouncedQuery.length >= 2 && accounts.length === 0 && (
        <div className="text-center py-12 text-gray-400">
          <Search className="w-10 h-10 mx-auto mb-3 opacity-30" />
          <p>No accounts found for "{debouncedQuery}"</p>
          <p className="text-sm mt-1">Try a different search term</p>
        </div>
      )}

      {/* Initial state */}
      {debouncedQuery.length < 2 && (
        <div className="text-center py-12 text-gray-400">
          <Search className="w-10 h-10 mx-auto mb-3 opacity-30" />
          <p>Enter at least 2 characters to search</p>
        </div>
      )}
    </div>
  )
}
