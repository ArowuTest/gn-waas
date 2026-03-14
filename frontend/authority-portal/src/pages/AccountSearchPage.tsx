/**
 * AccountSearchPage — Account lookup for GRA Officers and Field Supervisors.
 *
 * Search results are displayed as expandable cards. Clicking a card expands it
 * to show the full account detail inline (BUG-6 / NAV-3 fix: previously the
 * card had cursor-pointer style but no onClick — dead UI element).
 */
import { useState, useCallback } from 'react'
import {
  Search, User, MapPin, Droplets, AlertTriangle, Loader2,
  ChevronDown, ChevronUp, Phone, Hash, Calendar, BarChart3,
  CheckCircle, XCircle, Info,
} from 'lucide-react'
import { useAccountSearch } from '../hooks/useQueries'
import type { WaterAccount } from '../types'

// ── Inline detail panel shown when a card is expanded ──────────────────────
function AccountDetailPanel({ account }: { account: WaterAccount }) {
  const rows: Array<[string, React.ReactNode]> = [
    ['Account Number', account.gwl_account_number],
    ['Account Holder', account.account_holder_name],
    ['Category', account.category],
    ['Status', (
      <span className={`inline-block px-2 py-0.5 rounded-full text-xs font-semibold ${
        account.is_phantom_flagged ? 'bg-red-100 text-red-700' :
        account.status === 'INACTIVE' ? 'bg-orange-100 text-orange-700' :
        'bg-green-100 text-green-700'
      }`}>
        {account.is_phantom_flagged ? 'FLAGGED / PHANTOM' : account.status}
      </span>
    )],
    ['Meter Number', account.meter_number ?? <em className="text-orange-600">Not registered</em>],
    ['Address', account.address_line1],
    ['District', account.district_name ?? account.district_id ?? '—'],
    ['TIN', account.account_holder_tin ?? '—'],
    ['Avg. Consumption', account.monthly_avg_consumption
      ? `${Number(account.monthly_avg_consumption).toFixed(2)} m³/month`
      : '—'],
    ['Phantom Flagged', account.is_phantom_flagged
      ? <span className="flex items-center gap-1 text-red-600"><XCircle className="w-3.5 h-3.5" /> Yes — under investigation</span>
      : <span className="flex items-center gap-1 text-green-700"><CheckCircle className="w-3.5 h-3.5" /> No</span>],
    ['Account Created', account.created_at
      ? new Date(account.created_at).toLocaleDateString('en-GH', { year: 'numeric', month: 'long', day: 'numeric' })
      : '—'],
  ]

  return (
    <div className="mt-4 pt-4 border-t border-gray-100">
      <p className="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3 flex items-center gap-1.5">
        <Info className="w-3.5 h-3.5" /> Account Details
      </p>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-8 gap-y-1.5 text-sm">
        {rows.map(([label, value]) => (
          <div key={label} className="flex items-start gap-2">
            <span className="text-gray-500 min-w-[130px] flex-shrink-0">{label}:</span>
            <span className="text-gray-900 font-medium">{value}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

// ── AccountCard — expandable result card ────────────────────────────────────
function AccountCard({ account }: { account: WaterAccount }) {
  const [expanded, setExpanded] = useState(false)
  const hasAnomaly = account.is_phantom_flagged
  const statusLabel = hasAnomaly ? 'FLAGGED' : account.status

  return (
    <div
      role="button"
      tabIndex={0}
      aria-expanded={expanded}
      onClick={() => setExpanded(v => !v)}
      onKeyDown={e => (e.key === 'Enter' || e.key === ' ') && setExpanded(v => !v)}
      className={`bg-white rounded-xl border p-5 shadow-sm transition-colors cursor-pointer select-none ${
        expanded
          ? 'border-green-400 ring-1 ring-green-200'
          : 'border-gray-100 hover:border-green-200'
      }`}
    >
      <div className="flex items-start justify-between">
        <div className="flex items-start gap-4 flex-1 min-w-0">
          {/* Icon */}
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
          {/* Summary */}
          <div className="min-w-0">
            <div className="flex items-center gap-2 mb-1 flex-wrap">
              <span className="font-bold text-gray-900">{account.account_holder_name}</span>
              <span className="text-xs bg-gray-100 text-gray-600 px-2 py-0.5 rounded-full font-mono">
                {account.gwl_account_number}
              </span>
            </div>
            <div className="flex items-center gap-1 text-sm text-gray-500">
              <MapPin className="w-3 h-3 flex-shrink-0" />
              <span className="truncate">{account.address_line1}</span>
            </div>
            <div className="flex items-center gap-3 mt-2 text-xs text-gray-500 flex-wrap">
              <span className="bg-gray-100 px-2 py-0.5 rounded">{account.category}</span>
              {account.meter_number ? (
                <span className="flex items-center gap-1"><Hash className="w-3 h-3" />Meter: {account.meter_number}</span>
              ) : (
                <span className="text-orange-600 font-medium flex items-center gap-1"><XCircle className="w-3 h-3" />No meter</span>
              )}
              {account.monthly_avg_consumption > 0 && (
                <span className="flex items-center gap-1"><BarChart3 className="w-3 h-3" />{(account.monthly_avg_consumption ?? 0).toFixed(1)} m³/mo</span>
              )}
              {account.account_holder_tin && (
                <span className="flex items-center gap-1"><Phone className="w-3 h-3" />TIN: {account.account_holder_tin}</span>
              )}
              {account.created_at && (
                <span className="flex items-center gap-1"><Calendar className="w-3 h-3" />Since {new Date(account.created_at).getFullYear()}</span>
              )}
            </div>
          </div>
        </div>
        {/* Status + chevron */}
        <div className="flex items-center gap-2 flex-shrink-0 ml-3">
          <span className={`text-xs font-bold px-3 py-1 rounded-full ${
            hasAnomaly ? 'bg-red-100 text-red-700' :
            account.status === 'INACTIVE' ? 'bg-orange-100 text-orange-700' :
            'bg-green-100 text-green-700'
          }`}>
            {statusLabel}
          </span>
          {expanded
            ? <ChevronUp className="w-4 h-4 text-gray-400" />
            : <ChevronDown className="w-4 h-4 text-gray-400" />}
        </div>
      </div>

      {/* Expanded detail panel */}
      {expanded && <AccountDetailPanel account={account} />}
    </div>
  )
}

// ── Main page ───────────────────────────────────────────────────────────────
export default function AccountSearchPage() {
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const timerRef = useState<ReturnType<typeof setTimeout> | null>(null)

  // Debounce: cancel previous timer on each keystroke
  const handleQueryChange = useCallback((value: string) => {
    setQuery(value)
    if (timerRef[0]) clearTimeout(timerRef[0])
    timerRef[1](setTimeout(() => setDebouncedQuery(value), 400))
  }, [timerRef])

  const { data, isLoading, isFetching, isError } = useAccountSearch(debouncedQuery)
  const accounts = data?.accounts ?? []
  const total = data?.total ?? 0

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-black text-gray-900 mb-1">Account Search</h1>
        <p className="text-gray-500 text-sm">
          Search by account number, customer name, address, or meter number.
          Click any result to view full account details.
        </p>
      </div>

      {/* Search bar */}
      <div className="relative mb-6">
        <Search className="absolute left-4 top-3.5 w-4 h-4 text-gray-400" />
        <input
          type="text"
          value={query}
          onChange={e => handleQueryChange(e.target.value)}
          placeholder="Search accounts… (min. 2 characters)"
          className="w-full pl-11 pr-4 py-3 border border-gray-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-green-600 bg-white shadow-sm"
        />
        {isFetching && (
          <Loader2 className="absolute right-4 top-3.5 w-4 h-4 text-gray-400 animate-spin" />
        )}
      </div>

      {/* Results count */}
      {debouncedQuery.length >= 2 && !isLoading && (
        <p className="text-sm text-gray-500 mb-4">
          {total > 0
            ? `${total} account${total !== 1 ? 's' : ''} found — click a row to expand details`
            : 'No accounts found'}
        </p>
      )}

      {/* Loading */}
      {isLoading && debouncedQuery.length >= 2 && (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="w-8 h-8 animate-spin text-green-700" />
        </div>
      )}

      {/* Error */}
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

      {/* Empty */}
      {!isLoading && !isError && debouncedQuery.length >= 2 && accounts.length === 0 && (
        <div className="text-center py-12 text-gray-400">
          <Search className="w-10 h-10 mx-auto mb-3 opacity-30" />
          <p>No accounts found for "{debouncedQuery}"</p>
          <p className="text-sm mt-1">Try a different search term</p>
        </div>
      )}

      {/* Initial */}
      {debouncedQuery.length < 2 && (
        <div className="text-center py-12 text-gray-400">
          <Search className="w-10 h-10 mx-auto mb-3 opacity-30" />
          <p>Enter at least 2 characters to search</p>
        </div>
      )}
    </div>
  )
}
