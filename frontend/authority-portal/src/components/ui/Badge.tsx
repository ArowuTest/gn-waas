import { cn, getAlertLevelClass, getStatusClass } from '../../lib/utils'

interface BadgeProps {
  children: React.ReactNode
  variant?: 'red' | 'yellow' | 'green' | 'blue' | 'gray'
  className?: string
}

export function Badge({ children, variant = 'gray', className }: BadgeProps) {
  const variantMap = {
    red:    'badge-red',
    yellow: 'badge-yellow',
    green:  'badge-green',
    blue:   'badge-blue',
    gray:   'badge-gray',
  }
  return (
    <span className={cn('badge', variantMap[variant], className)}>
      {children}
    </span>
  )
}

export function AlertLevelBadge({ level }: { level: string }) {
  return (
    <span className={cn('badge', getAlertLevelClass(level))}>
      {level}
    </span>
  )
}

export function StatusBadge({ status }: { status: string }) {
  return (
    <span className={cn('badge', getStatusClass(status))}>
      {status.replace(/_/g, ' ')}
    </span>
  )
}

export function GRAStatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    PENDING:   'badge-yellow',
    SUBMITTED: 'badge-blue',
    SIGNED:    'badge-green',
    FAILED:    'badge-red',
  }
  return (
    <span className={cn('badge', styles[status] || 'badge-gray')}>
      {status === 'SIGNED' ? '✓ GRA Signed' : status}
    </span>
  )
}
