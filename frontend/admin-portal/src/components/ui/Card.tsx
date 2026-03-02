import { type ReactNode } from 'react'
import { cn } from '../../lib/utils'

interface CardProps {
  children: ReactNode
  className?: string
  title?: string
  subtitle?: string
  action?: ReactNode
  noPadding?: boolean
  gradient?: boolean
}

export function Card({ children, className, title, subtitle, action, noPadding, gradient }: CardProps) {
  return (
    <div className={cn(
      'bg-white rounded-2xl border border-gray-100 shadow-card overflow-hidden',
      gradient && 'bg-gradient-to-br from-white to-gray-50/50',
      className
    )}>
      {(title || action) && (
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
          <div>
            {title && <h3 className="text-sm font-semibold text-gray-900">{title}</h3>}
            {subtitle && <p className="text-xs text-gray-500 mt-0.5">{subtitle}</p>}
          </div>
          {action && <div className="flex items-center gap-2">{action}</div>}
        </div>
      )}
      <div className={noPadding ? '' : 'p-6'}>{children}</div>
    </div>
  )
}

interface StatCardProps {
  title: string
  value: string | number
  subtitle?: string
  icon?: ReactNode
  trend?: { value: number; label: string }
  variant?: 'default' | 'danger' | 'warning' | 'success' | 'info' | 'brand'
  onClick?: () => void
}

const variantConfig = {
  default: { border: '', icon: 'bg-gray-100 text-gray-500', value: 'text-gray-900' },
  danger:  { border: 'border-t-4 border-t-red-500', icon: 'bg-red-50 text-red-600', value: 'text-red-700' },
  warning: { border: 'border-t-4 border-t-amber-500', icon: 'bg-amber-50 text-amber-600', value: 'text-amber-700' },
  success: { border: 'border-t-4 border-t-emerald-500', icon: 'bg-emerald-50 text-emerald-600', value: 'text-emerald-700' },
  info:    { border: 'border-t-4 border-t-blue-500', icon: 'bg-blue-50 text-blue-600', value: 'text-blue-700' },
  brand:   { border: 'border-t-4 border-t-brand-600', icon: 'bg-brand-50 text-brand-600', value: 'text-brand-700' },
}

export function StatCard({ title, value, subtitle, icon, trend, variant = 'default', onClick }: StatCardProps) {
  const cfg = variantConfig[variant]

  return (
    <div
      className={cn(
        'bg-white rounded-2xl border border-gray-100 shadow-card p-5 transition-all duration-200',
        cfg.border,
        onClick && 'cursor-pointer hover:shadow-card-hover hover:-translate-y-0.5'
      )}
      onClick={onClick}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex-1 min-w-0">
          <p className="text-xs font-semibold text-gray-500 uppercase tracking-wider truncate">{title}</p>
          <p className={cn('text-2xl font-bold mt-1.5 tabular-nums', cfg.value)}>{value}</p>
          {subtitle && <p className="text-xs text-gray-400 mt-1 truncate">{subtitle}</p>}
          {trend && (
            <div className={cn(
              'flex items-center gap-1 mt-2 text-xs font-semibold',
              trend.value >= 0 ? 'text-emerald-600' : 'text-red-600'
            )}>
              <span>{trend.value >= 0 ? '↑' : '↓'} {Math.abs(trend.value)}%</span>
              <span className="text-gray-400 font-normal">{trend.label}</span>
            </div>
          )}
        </div>
        {icon && (
          <div className={cn('p-2.5 rounded-xl flex-shrink-0', cfg.icon)}>
            {icon}
          </div>
        )}
      </div>
    </div>
  )
}
