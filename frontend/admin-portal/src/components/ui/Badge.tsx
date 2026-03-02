import { cn } from '../../lib/utils'

// ── Status Badge ──────────────────────────────────────────────────────────────
const STATUS_CONFIG: Record<string, { label: string; className: string; dot: string }> = {
  PENDING:     { label: 'Pending',     className: 'bg-amber-50 text-amber-700 ring-1 ring-amber-200',   dot: 'bg-amber-500' },
  ASSIGNED:    { label: 'Assigned',    className: 'bg-blue-50 text-blue-700 ring-1 ring-blue-200',      dot: 'bg-blue-500' },
  IN_PROGRESS: { label: 'In Progress', className: 'bg-purple-50 text-purple-700 ring-1 ring-purple-200', dot: 'bg-purple-500' },
  COMPLETED:   { label: 'Completed',   className: 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200', dot: 'bg-emerald-500' },
  CLOSED:      { label: 'Closed',      className: 'bg-gray-100 text-gray-600 ring-1 ring-gray-200',     dot: 'bg-gray-400' },
  OPEN:        { label: 'Open',        className: 'bg-red-50 text-red-700 ring-1 ring-red-200',         dot: 'bg-red-500' },
  ACKNOWLEDGED:{ label: 'Acknowledged',className: 'bg-blue-50 text-blue-700 ring-1 ring-blue-200',      dot: 'bg-blue-500' },
  RESOLVED:    { label: 'Resolved',    className: 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200', dot: 'bg-emerald-500' },
  FALSE_POSITIVE: { label: 'False Positive', className: 'bg-gray-100 text-gray-600 ring-1 ring-gray-200', dot: 'bg-gray-400' },
}

export function StatusBadge({ status }: { status: string }) {
  const cfg = STATUS_CONFIG[status] ?? { label: status, className: 'bg-gray-100 text-gray-600 ring-1 ring-gray-200', dot: 'bg-gray-400' }
  return (
    <span className={cn('inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold', cfg.className)}>
      <span className={cn('w-1.5 h-1.5 rounded-full flex-shrink-0', cfg.dot)} />
      {cfg.label}
    </span>
  )
}

// ── GRA Status Badge ──────────────────────────────────────────────────────────
const GRA_STATUS_CONFIG: Record<string, { label: string; className: string }> = {
  PENDING:   { label: 'Pending',   className: 'bg-gray-100 text-gray-600 ring-1 ring-gray-200' },
  SUBMITTED: { label: 'Submitted', className: 'bg-blue-50 text-blue-700 ring-1 ring-blue-200' },
  SIGNED:    { label: 'GRA Signed', className: 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200' },
  REJECTED:  { label: 'Rejected',  className: 'bg-red-50 text-red-700 ring-1 ring-red-200' },
  LOCKED:    { label: 'Locked',    className: 'bg-purple-50 text-purple-700 ring-1 ring-purple-200' },
}

export function GRAStatusBadge({ status }: { status?: string }) {
  if (!status) return <span className="text-gray-400 text-xs">—</span>
  const cfg = GRA_STATUS_CONFIG[status] ?? { label: status, className: 'bg-gray-100 text-gray-600 ring-1 ring-gray-200' }
  return (
    <span className={cn('inline-flex items-center px-2.5 py-1 rounded-full text-xs font-semibold', cfg.className)}>
      {cfg.label}
    </span>
  )
}

// ── Alert Level Badge ─────────────────────────────────────────────────────────
const ALERT_CONFIG: Record<string, { label: string; className: string; dot: string }> = {
  CRITICAL: { label: 'Critical', className: 'bg-red-50 text-red-700 ring-1 ring-red-200',       dot: 'bg-red-500' },
  HIGH:     { label: 'High',     className: 'bg-orange-50 text-orange-700 ring-1 ring-orange-200', dot: 'bg-orange-500' },
  MEDIUM:   { label: 'Medium',   className: 'bg-amber-50 text-amber-700 ring-1 ring-amber-200',  dot: 'bg-amber-500' },
  LOW:      { label: 'Low',      className: 'bg-blue-50 text-blue-700 ring-1 ring-blue-200',     dot: 'bg-blue-500' },
  INFO:     { label: 'Info',     className: 'bg-gray-100 text-gray-600 ring-1 ring-gray-200',    dot: 'bg-gray-400' },
}

export function AlertLevelBadge({ level }: { level: string }) {
  const cfg = ALERT_CONFIG[level] ?? { label: level, className: 'bg-gray-100 text-gray-600 ring-1 ring-gray-200', dot: 'bg-gray-400' }
  return (
    <span className={cn('inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold', cfg.className)}>
      <span className={cn('w-1.5 h-1.5 rounded-full flex-shrink-0', cfg.dot)} />
      {cfg.label}
    </span>
  )
}

// ── NRW Grade Badge ───────────────────────────────────────────────────────────
const NRW_GRADE_CONFIG: Record<string, { className: string }> = {
  A: { className: 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200' },
  B: { className: 'bg-green-50 text-green-700 ring-1 ring-green-200' },
  C: { className: 'bg-amber-50 text-amber-700 ring-1 ring-amber-200' },
  D: { className: 'bg-orange-50 text-orange-700 ring-1 ring-orange-200' },
  F: { className: 'bg-red-50 text-red-700 ring-1 ring-red-200' },
}

export function NRWGradeBadge({ grade }: { grade: string }) {
  const cfg = NRW_GRADE_CONFIG[grade] ?? { className: 'bg-gray-100 text-gray-600 ring-1 ring-gray-200' }
  return (
    <span className={cn('inline-flex items-center px-2.5 py-1 rounded-full text-xs font-bold', cfg.className)}>
      Grade {grade}
    </span>
  )
}
