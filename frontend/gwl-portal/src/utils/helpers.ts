import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { GWLStatus, Severity, FlagType } from '../types';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatGHS(amount: number): string {
  return new Intl.NumberFormat('en-GH', {
    style: 'currency',
    currency: 'GHS',
    minimumFractionDigits: 2,
  }).format(amount);
}

export function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-GB', {
    day: '2-digit', month: 'short', year: 'numeric',
  });
}

export function formatDateTime(dateStr: string): string {
  return new Date(dateStr).toLocaleString('en-GB', {
    day: '2-digit', month: 'short', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  });
}

// ── Status display helpers ────────────────────────────────────────────────────

export const GWL_STATUS_LABELS: Record<GWLStatus, string> = {
  PENDING_REVIEW:          'Pending Review',
  UNDER_INVESTIGATION:     'Under Investigation',
  FIELD_ASSIGNED:          'Field Assigned',
  EVIDENCE_SUBMITTED:      'Evidence Submitted',
  APPROVED_FOR_CORRECTION: 'Approved for Correction',
  DISPUTED:                'Disputed',
  CORRECTED:               'Corrected',
  CLOSED:                  'Closed',
};

export const GWL_STATUS_COLORS: Record<GWLStatus, string> = {
  PENDING_REVIEW:          'bg-yellow-100 text-yellow-800 border-yellow-200',
  UNDER_INVESTIGATION:     'bg-blue-100 text-blue-800 border-blue-200',
  FIELD_ASSIGNED:          'bg-purple-100 text-purple-800 border-purple-200',
  EVIDENCE_SUBMITTED:      'bg-indigo-100 text-indigo-800 border-indigo-200',
  APPROVED_FOR_CORRECTION: 'bg-orange-100 text-orange-800 border-orange-200',
  DISPUTED:                'bg-gray-100 text-gray-700 border-gray-200',
  CORRECTED:               'bg-green-100 text-green-800 border-green-200',
  CLOSED:                  'bg-gray-100 text-gray-500 border-gray-200',
};

export const SEVERITY_COLORS: Record<Severity, string> = {
  CRITICAL: 'bg-red-100 text-red-800 border-red-200',
  HIGH:     'bg-orange-100 text-orange-800 border-orange-200',
  MEDIUM:   'bg-yellow-100 text-yellow-800 border-yellow-200',
  LOW:      'bg-green-100 text-green-800 border-green-200',
};

export const FLAG_TYPE_LABELS: Record<FlagType, string> = {
  BILLING_VARIANCE:   'Billing Variance',
  CATEGORY_MISMATCH:  'Category Mismatch',
  OVERBILLING:        'Overbilling',
  PHANTOM_METER:      'Phantom Meter',
  NRW_SPIKE:          'NRW Spike',
  NIGHT_FLOW_ANOMALY: 'Night Flow Anomaly',
  METER_TAMPERING:    'Meter Tampering',
};

export const FLAG_TYPE_COLORS: Record<FlagType, string> = {
  BILLING_VARIANCE:   'bg-amber-100 text-amber-800',
  CATEGORY_MISMATCH:  'bg-violet-100 text-violet-800',
  OVERBILLING:        'bg-rose-100 text-rose-800',
  PHANTOM_METER:      'bg-red-100 text-red-800',
  NRW_SPIKE:          'bg-orange-100 text-orange-800',
  NIGHT_FLOW_ANOMALY: 'bg-blue-100 text-blue-800',
  METER_TAMPERING:    'bg-red-100 text-red-900',
};

// ── CSV Export ────────────────────────────────────────────────────────────────
export function exportToCSV<T>(
  data: T[],
  columns: { header: string; accessor: (row: T) => string | number }[],
  filename: string
) {
  const header = columns.map((c) => c.header).join(',');
  const rows = data.map((row) =>
    columns.map((c) => {
      const val = c.accessor(row);
      return typeof val === 'string' && val.includes(',') ? `"${val}"` : val;
    }).join(',')
  );
  const csv = [header, ...rows].join('\n');
  const blob = new Blob([csv], { type: 'text/csv' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
