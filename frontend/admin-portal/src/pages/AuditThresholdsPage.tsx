import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '../utils/api'

// ─── Types ────────────────────────────────────────────────────────────────────

interface SystemConfig {
  id: string
  config_key: string
  config_value: string
  config_type: string   // 'number' | 'string' | 'boolean' | 'json'
  description: string
  category: string
  updated_at: string
}

// Human-readable metadata for each config key
const CONFIG_META: Record<string, { label: string; hint: string; unit?: string; icon: string }> = {
  // Sentinel thresholds
  'sentinel.variance_threshold_pct':    { label: 'Bill Variance Alert Threshold', hint: 'Trigger audit when shadow bill differs from GWL bill by more than this %', unit: '%', icon: '⚡' },
  'sentinel.critical_variance_pct':     { label: 'Critical Variance Threshold', hint: 'Escalate to CRITICAL alert level above this variance %', unit: '%', icon: '🚨' },
  'sentinel.night_flow_threshold_m3':   { label: 'Night Flow Alert Threshold', hint: 'Flag district if 2–4 AM bulk flow exceeds this value (m³/hr)', unit: 'm³/hr', icon: '🌙' },
  'sentinel.phantom_meter_days':        { label: 'Phantom Meter Detection Window', hint: 'Flag accounts with no readings for this many consecutive days', unit: 'days', icon: '👻' },
  'sentinel.ghost_account_months':      { label: 'Ghost Account Detection Window', hint: 'Flag accounts billed but with zero consumption for this many months', unit: 'months', icon: '💀' },
  'sentinel.category_mismatch_pct':     { label: 'Category Mismatch Tolerance', hint: 'Flag accounts where actual usage deviates from category average by this %', unit: '%', icon: '🔀' },
  // NRW thresholds
  'nrw.target_pct':                     { label: 'NRW Target (%)', hint: 'IWA target NRW percentage for Ghana (PURC guideline)', unit: '%', icon: '🎯' },
  'nrw.critical_pct':                   { label: 'NRW Critical Threshold (%)', hint: 'Districts above this NRW % are flagged as critical', unit: '%', icon: '🔴' },
  'nrw.water_tariff_ghs_m3':            { label: 'Water Tariff for Recovery Calc', hint: 'GHS per m³ used to estimate revenue recovery potential', unit: 'GHS/m³', icon: '💰' },
  // Audit settings
  'audit.auto_assign_enabled':          { label: 'Auto-Assign Field Jobs', hint: 'Automatically assign anomaly flags to available field officers', unit: '', icon: '🤖' },
  'audit.max_jobs_per_officer':         { label: 'Max Active Jobs per Officer', hint: 'Maximum concurrent open jobs assigned to a single field officer', unit: 'jobs', icon: '👷' },
  'audit.evidence_photo_required':      { label: 'Require Photo Evidence', hint: 'Field officers must upload meter photo before closing a job', unit: '', icon: '📷' },
  'audit.gps_fence_radius_m':           { label: 'GPS Fence Radius', hint: 'Maximum distance (metres) from meter GPS coordinates for evidence capture', unit: 'm', icon: '📍' },
  // GRA settings
  'gra.sandbox_mode':                   { label: 'GRA Sandbox Mode', hint: 'Use GRA VSDC sandbox API (disable for production)', unit: '', icon: '🧪' },
  'gra.invoice_vat_rate_pct':           { label: 'VAT Rate for Audit Invoices', hint: 'VAT percentage applied to audit recovery invoices', unit: '%', icon: '🧾' },
  // CDC settings
  'cdc.sync_interval_minutes':          { label: 'CDC Sync Interval', hint: 'How often the CDC ingestor syncs from GWL replica database', unit: 'min', icon: '🔄' },
  'cdc.batch_size':                     { label: 'CDC Batch Size', hint: 'Number of records processed per CDC sync batch', unit: 'rows', icon: '📦' },
}

const CATEGORY_ICONS: Record<string, string> = {
  sentinel: '🛡️',
  nrw: '💧',
  audit: '📋',
  gra: '🏛️',
  cdc: '🔄',
}

// ─── Inline Edit Row ──────────────────────────────────────────────────────────

function ConfigRow({
  cfg,
  onSave,
}: {
  cfg: SystemConfig
  onSave: (key: string, value: string) => void
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(cfg.config_value)
  const meta = CONFIG_META[cfg.config_key]

  const isBool = cfg.config_type === 'boolean' || cfg.config_value === 'true' || cfg.config_value === 'false'

  const handleSave = () => {
    onSave(cfg.config_key, draft)
    setEditing(false)
  }

  const handleCancel = () => {
    setDraft(cfg.config_value)
    setEditing(false)
  }

  return (
    <tr className={editing ? 'row-editing' : ''}>
      <td>
        <div className="config-key-cell">
          <span className="config-icon">{meta?.icon ?? '⚙️'}</span>
          <div>
            <div className="config-label">{meta?.label ?? cfg.config_key}</div>
            <div className="config-key-raw">{cfg.config_key}</div>
          </div>
        </div>
      </td>
      <td className="config-hint">{meta?.hint ?? cfg.description}</td>
      <td>
        {editing ? (
          <div className="edit-cell">
            {isBool ? (
              <select value={draft} onChange={e => setDraft(e.target.value)} className="edit-select">
                <option value="true">Enabled</option>
                <option value="false">Disabled</option>
              </select>
            ) : (
              <div className="edit-input-wrap">
                <input
                  className="edit-input"
                  value={draft}
                  onChange={e => setDraft(e.target.value)}
                  type={cfg.config_type === 'number' ? 'number' : 'text'}
                  autoFocus
                />
                {meta?.unit && <span className="edit-unit">{meta.unit}</span>}
              </div>
            )}
          </div>
        ) : (
          <div className="value-cell">
            <span className="config-value">
              {isBool
                ? (cfg.config_value === 'true' ? '✅ Enabled' : '❌ Disabled')
                : cfg.config_value}
            </span>
            {meta?.unit && !isBool && <span className="value-unit">{meta.unit}</span>}
          </div>
        )}
      </td>
      <td className="updated-cell">
        {new Date(cfg.updated_at).toLocaleDateString('en-GH')}
      </td>
      <td>
        {editing ? (
          <div className="action-buttons">
            <button className="btn-save" onClick={handleSave}>✓ Save</button>
            <button className="btn-cancel" onClick={handleCancel}>✕</button>
          </div>
        ) : (
          <button className="btn-icon" onClick={() => setEditing(true)} title="Edit">✏️</button>
        )}
      </td>
    </tr>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

const CATEGORIES = ['sentinel', 'nrw', 'audit', 'gra', 'cdc']

export default function AuditThresholdsPage() {
  const queryClient = useQueryClient()
  const [activeCategory, setActiveCategory] = useState('sentinel')
  const [saveStatus, setSaveStatus] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle')

  const { data: configs = [], isLoading } = useQuery<SystemConfig[]>({
    queryKey: ['system-config', activeCategory],
    queryFn: async () => {
      const res = await apiClient.get(`/api/v1/config/${activeCategory}`)
      return res.data.data ?? []
    },
  })

  const updateConfig = useMutation({
    mutationFn: ({ key, value }: { key: string; value: string }) =>
      apiClient.patch(`/api/v1/config/${key}`, { value }),
    onMutate: () => setSaveStatus('saving'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['system-config'] })
      setSaveStatus('saved')
      setTimeout(() => setSaveStatus('idle'), 2000)
    },
    onError: () => setSaveStatus('error'),
  })

  const handleSave = (key: string, value: string) => {
    updateConfig.mutate({ key, value })
  }

  return (
    <div className="page">
      <style>{pageStyles}</style>

      {/* Header */}
      <div className="page-header">
        <div>
          <h1 className="page-title">Audit Thresholds & System Settings</h1>
          <p className="page-subtitle">
            Configure sentinel detection thresholds, NRW targets, audit rules, and integration settings
          </p>
        </div>
        {saveStatus === 'saving' && <span className="save-status saving">⏳ Saving…</span>}
        {saveStatus === 'saved'  && <span className="save-status saved">✅ Saved</span>}
        {saveStatus === 'error'  && <span className="save-status error">❌ Save failed</span>}
      </div>

      {/* Warning banner */}
      <div className="warning-banner">
        ⚠️ <strong>Production settings.</strong> Changes take effect immediately and affect all active sentinel scans and field operations.
        Only SYSTEM_ADMIN users can modify these values.
      </div>

      {/* Category tabs */}
      <div className="category-tabs">
        {CATEGORIES.map(cat => (
          <button
            key={cat}
            className={`cat-tab ${activeCategory === cat ? 'cat-tab--active' : ''}`}
            onClick={() => setActiveCategory(cat)}
          >
            {CATEGORY_ICONS[cat] ?? '⚙️'} {cat.charAt(0).toUpperCase() + cat.slice(1)}
          </button>
        ))}
      </div>

      {/* Config table */}
      {isLoading ? (
        <div className="loading">Loading configuration…</div>
      ) : (
        <div className="table-wrapper">
          <table className="data-table">
            <thead>
              <tr>
                <th style={{ width: '28%' }}>Setting</th>
                <th style={{ width: '40%' }}>Description</th>
                <th style={{ width: '16%' }}>Current Value</th>
                <th style={{ width: '10%' }}>Last Updated</th>
                <th style={{ width: '6%' }}>Edit</th>
              </tr>
            </thead>
            <tbody>
              {configs.map(cfg => (
                <ConfigRow key={cfg.config_key} cfg={cfg} onSave={handleSave} />
              ))}
              {configs.length === 0 && (
                <tr>
                  <td colSpan={5} className="empty-row">
                    No configuration found for category "{activeCategory}"
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// ─── Styles ───────────────────────────────────────────────────────────────────
const pageStyles = `
  .page { padding: 24px; max-width: 1200px; margin: 0 auto; }
  .page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }
  .page-title { font-size: 24px; font-weight: 700; color: #111827; margin: 0 0 4px; }
  .page-subtitle { font-size: 14px; color: #6b7280; margin: 0; }

  .save-status { font-size: 13px; font-weight: 600; padding: 6px 12px; border-radius: 8px; }
  .save-status.saving { background: #fef3c7; color: #92400e; }
  .save-status.saved  { background: #dcfce7; color: #166534; }
  .save-status.error  { background: #fee2e2; color: #991b1b; }

  .warning-banner { background: #fffbeb; border: 1px solid #fcd34d; border-radius: 10px;
    padding: 12px 16px; font-size: 13px; color: #78350f; margin-bottom: 20px; }

  .category-tabs { display: flex; gap: 4px; margin-bottom: 16px; background: #f3f4f6;
    padding: 4px; border-radius: 10px; width: fit-content; }
  .cat-tab { padding: 8px 16px; border: none; border-radius: 8px; font-size: 13px;
    font-weight: 600; cursor: pointer; background: transparent; color: #6b7280; transition: all 0.15s; }
  .cat-tab:hover { background: #e5e7eb; color: #374151; }
  .cat-tab--active { background: #fff; color: #2e7d32; box-shadow: 0 1px 4px rgba(0,0,0,0.1); }

  .table-wrapper { background: #fff; border-radius: 12px; overflow: hidden; border: 1px solid #e5e7eb; }
  .data-table { width: 100%; border-collapse: collapse; }
  .data-table th { background: #f9fafb; padding: 11px 16px; text-align: left; font-size: 11px;
    font-weight: 600; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;
    border-bottom: 1px solid #e5e7eb; }
  .data-table td { padding: 14px 16px; border-bottom: 1px solid #f3f4f6; vertical-align: middle; }
  .data-table tr:last-child td { border-bottom: none; }
  .row-editing td { background: #f0fdf4; }

  .config-key-cell { display: flex; align-items: flex-start; gap: 10px; }
  .config-icon { font-size: 18px; margin-top: 2px; }
  .config-label { font-size: 13px; font-weight: 600; color: #111827; }
  .config-key-raw { font-size: 10px; color: #9ca3af; font-family: monospace; margin-top: 2px; }
  .config-hint { font-size: 12px; color: #6b7280; line-height: 1.5; }

  .value-cell { display: flex; align-items: center; gap: 6px; }
  .config-value { font-size: 14px; font-weight: 700; color: #111827; }
  .value-unit { font-size: 11px; color: #9ca3af; }
  .updated-cell { font-size: 12px; color: #9ca3af; }

  .edit-cell { display: flex; align-items: center; gap: 6px; }
  .edit-input-wrap { display: flex; align-items: center; gap: 6px; }
  .edit-input { padding: 6px 10px; border: 2px solid #2e7d32; border-radius: 6px;
    font-size: 14px; font-weight: 600; width: 100px; outline: none; }
  .edit-select { padding: 6px 10px; border: 2px solid #2e7d32; border-radius: 6px;
    font-size: 13px; outline: none; }
  .edit-unit { font-size: 11px; color: #6b7280; }

  .action-buttons { display: flex; gap: 6px; }
  .btn-save { padding: 5px 12px; background: #2e7d32; color: #fff; border: none;
    border-radius: 6px; font-size: 12px; font-weight: 600; cursor: pointer; }
  .btn-save:hover { background: #1b5e20; }
  .btn-cancel { padding: 5px 10px; background: #f3f4f6; color: #374151; border: none;
    border-radius: 6px; font-size: 12px; cursor: pointer; }
  .btn-icon { width: 30px; height: 30px; border: none; border-radius: 6px; cursor: pointer;
    font-size: 14px; background: #f3f4f6; }
  .btn-icon:hover { background: #e5e7eb; }
  .empty-row { text-align: center; color: #9ca3af; padding: 40px !important; }
  .loading { text-align: center; padding: 40px; color: #6b7280; }
`
