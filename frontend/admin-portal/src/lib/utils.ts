import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'
import { format, formatDistanceToNow } from 'date-fns'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatCurrency(amount: number, currency = 'GHS'): string {
  return new Intl.NumberFormat('en-GH', {
    style: 'currency',
    currency,
    minimumFractionDigits: 2,
  }).format(amount)
}

export function formatNumber(value: number, decimals = 2): string {
  return new Intl.NumberFormat('en-GH', {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  }).format(value)
}

export function formatDate(date: string | Date): string {
  return format(new Date(date), 'dd MMM yyyy')
}

export function formatDateTime(date: string | Date): string {
  return format(new Date(date), 'dd MMM yyyy, HH:mm')
}

export function formatRelativeTime(date: string | Date): string {
  return formatDistanceToNow(new Date(date), { addSuffix: true })
}

export function getAlertLevelClass(level: string): string {
  switch (level.toUpperCase()) {
    case 'CRITICAL': return 'badge-red'
    case 'HIGH':     return 'badge-red'
    case 'MEDIUM':   return 'badge-yellow'
    case 'LOW':      return 'badge-blue'
    default:         return 'badge-gray'
  }
}

export function getStatusClass(status: string): string {
  switch (status.toUpperCase()) {
    case 'OPEN':        return 'badge-red'
    case 'IN_PROGRESS': return 'badge-yellow'
    case 'RESOLVED':    return 'badge-green'
    case 'CLOSED':      return 'badge-gray'
    case 'PENDING':     return 'badge-yellow'
    case 'COMPLETED':   return 'badge-green'
    case 'ASSIGNED':    return 'badge-blue'
    default:            return 'badge-gray'
  }
}

export function getDataConfidenceGrade(score: number): { grade: string; label: string; className: string } {
  if (score >= 90) return { grade: 'A', label: 'Excellent', className: 'grade-a' }
  if (score >= 75) return { grade: 'B', label: 'Good',      className: 'grade-b' }
  if (score >= 60) return { grade: 'C', label: 'Fair',      className: 'grade-c' }
  if (score >= 40) return { grade: 'D', label: 'Poor',      className: 'grade-d' }
  return { grade: 'F', label: 'Unreliable', className: 'grade-f' }
}

export function truncate(str: string, maxLength: number): string {
  if (str.length <= maxLength) return str
  return str.slice(0, maxLength) + '...'
}
