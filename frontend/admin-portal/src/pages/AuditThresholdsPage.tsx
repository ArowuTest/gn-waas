import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '../lib/api-client'

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

// FE-FIX-02: CONFIG_META keys must match the actual config_key values in the
// system_config table (seeded by database/seeds/001_system_config.sql).
// Previous keys (sentinel.variance_threshold_pct, nrw.*, audit.*) did not exist
// in the DB and caused the table to render with no human-readable labels.
const CONFIG_META: Record<string, { label: string; hint: string; unit?: string; icon: string }> = {
  // Sentinel thresholds (category: SENTINEL)
  'sentinel.shadow_bill_variance_pct':  { label: 'Shadow Bill Variance Threshold', hint: 'Trigger audit when shadow bill differs from GWL actual bill by more than this %', unit: '%', icon: '⚡' },
  'sentinel.night_flow_pct_of_daily':   { label: 'Night Flow Alert Threshold', hint: 'Flag district if 2–4 AM flow exceeds this % of daily average', unit: '%', icon: '🌙' },
  'sentinel.phantom_meter_months':      { label: 'Phantom Meter Detection Window', hint: 'Flag accounts with identical readings for this many consecutive months', unit: 'months', icon: '👻' },
  'sentinel.district_imbalance_pct':    { label: 'District Imbalance Threshold', hint: 'Flag district when production vs billing imbalance exceeds this %', unit: '%', icon: '⚖️' },
  'sentinel.rationing_drop_pct':        { label: 'Rationing Drop Threshold', hint: 'Expected consumption drop during rationing period (%)', unit: '%', icon: '🚰' },
  'sentinel.min_consumption_flag_m3':   { label: 'Minimum Consumption Flag', hint: 'Flag accounts with monthly consumption below this value', unit: 'm³', icon: '💀' },
  // Field operations (category: FIELD)
  'field.gps_fence_radius_m':           { label: 'GPS Fence Radius', hint: 'Maximum distance (metres) from meter GPS coordinates for evidence capture', unit: 'm', icon: '📍' },
  'field.ocr_conflict_tolerance_pct':   { label: 'OCR Conflict Tolerance', hint: 'Flag if OCR reading differs from manual entry by more than this %', unit: '%', icon: '📷' },
  'field.max_photo_age_minutes':        { label: 'Max Photo Age', hint: 'Maximum age of meter photo before rejection (minutes)', unit: 'min', icon: '🕐' },
  'field.require_biometric':            { label: 'Require Biometric Verification', hint: 'Require biometric verification for field officers', unit: '', icon: '🔐' },
  'field.blind_audit_default':          { label: 'Blind Audit Default', hint: 'Enable blind audit mode by default (officer sees GPS only, not account details)', unit: '', icon: '🙈' },
  'field.require_surroundings_photo':   { label: 'Require Surroundings Photo', hint: 'Require surroundings photo in addition to meter face photo', unit: '', icon: '🖼️' },
  'field.sync_interval_seconds':        { label: 'Mobile Sync Interval', hint: 'How often the Flutter app syncs pending submissions (seconds)', unit: 's', icon: '🔄' },
  // GRA compliance (category: GRA)
  'gra.api_base_url':                   { label: 'GRA VSDC API URL', hint: 'GRA VSDC API base URL for e-VAT signing', unit: '', icon: '🏛️' },
  'gra.api_timeout_seconds':            { label: 'GRA API Timeout', hint: 'GRA API request timeout (seconds)', unit: 's', icon: '⏱️' },
  'gra.max_retry_attempts':             { label: 'GRA Max Retries', hint: 'Maximum GRA API retry attempts before marking FAILED', unit: '', icon: '🔁' },
  'gra.retry_delay_seconds':            { label: 'GRA Retry Delay', hint: 'Delay between GRA API retry attempts (seconds)', unit: 's', icon: '⏳' },
  'gra.vat_threshold_ghs':             { label: 'VAT Signing Threshold', hint: 'Minimum bill amount requiring GRA VAT signing (GHS)', unit: 'GHS', icon: '🧾' },
  // CDC synchronisation (category: CDC)
  'cdc.sync_interval_minutes':          { label: 'CDC Sync Interval', hint: 'How often the CDC ingestor syncs from GWL replica database', unit: 'min', icon: '🔄' },
  'cdc.max_lag_minutes':                { label: 'CDC Max Lag', hint: 'Maximum acceptable CDC lag before alert (minutes)', unit: 'min', icon: '⚠️' },
  'cdc.batch_size':                     { label: 'CDC Batch Size', hint: 'Number of records processed per CDC sync batch', unit: 'rows', icon: '📦' },
  // Mobile app (category: MOBILE)
  'mobile.app_min_version':             { label: 'Minimum App Version', hint: 'Minimum Flutter app version required (older versions prompted to update)', unit: '', icon: '📱' },
  'mobile.app_latest_version':          { label: 'Latest App Version', hint: 'Latest Flutter app version (displayed in About screen)', unit: '', icon: '🆕' },
  'mobile.force_update':                { label: 'Force Update', hint: 'Block app usage until officer updates to minimum version', unit: '', icon: '🚫' },
  'mobile.maintenance_mode':            { label: 'Maintenance Mode', hint: 'Disable field officer login and show maintenance message', unit: '', icon: '🔧' },
  'mobile.maintenance_message':         { label: 'Maintenance Message', hint: 'Message shown to field officers during maintenance mode', unit: '', icon: '💬' },
  // Business model (category: BUSINESS)
  'business.success_fee_rate_pct':      { label: 'Success Fee Rate', hint: 'Success fee rate on recovered revenue (%)', unit: '%', icon: '💰' },
  'business.company_name':              { label: 'Managing Company Name', hint: 'Name of the managed-service operator', unit: '', icon: '🏢' },
  'business.company_tin':               { label: 'Managing Company TIN', hint: 'GRA TIN of the managed-service operator', unit: '', icon: '🪪' },
}

// FE-FIX-02: CATEGORY_ICONS must match actual DB categories (SENTINEL, FIELD, GRA, CDC, MOBILE, BUSINESS)
// 'nrw' and 'audit' are not DB categories — removed.
const CATEGORY_ICONS: Record<string, string> = {
  sentinel: '🛡️',
  field:    '👷',
  gra:      '🏛️',
  cdc:      '🔄',
  mobile:   '📱',
  business: '💰',
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
  const [rowSaving, setRowSaving] = useState(false)
  const meta = CONFIG_META[cfg.config_key]

  const isBool = cfg.config_value === 'true' || cfg.config_value === 'false' || cfg.config_type === 'boolean'

  const handleSave = () => {
    setRowSaving(true)
    // onSave is sync from this component's perspective; parent mutation handles
    // its own async lifecycle. Reset rowSaving after a short tick so the button
    // re-enables once the row closes.
    try {
      onSave(cfg.config_key, draft)
    } finally {
      setEditing(false)
      setRowSaving(false)
    }
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
            <button className="btn-save" onClick={handleSave} disabled={rowSaving}>
              {rowSaving ? '⏳ Saving…' : '✓ Save'}
            </button>
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

// FE-FIX-02: Categories match actual DB system_config category values (case-insensitive via UPPER())
const CATEGORIES = ['sentinel', 'field', 'gra', 'cdc', 'mobile', 'business']

export default function AuditThresholdsPage() {
  const queryClient = useQueryClient()
  const [activeCategory, setActiveCategory] = useState('sentinel')
  const [saveStatus, setSaveStatus] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle')

  const { data: configs = [], isLoading } = useQuery<SystemConfig[]>({
    queryKey: ['system-config', activeCategory],
    queryFn: async () => {
      const res = await apiClient.get(`/config/${activeCategory}`)
      return res.data.data ?? []
    },
  })

  const updateConfig = useMutation({
    mutationFn: ({ key, value }: { key: string; value: string }) =>
      apiClient.patch(`/config/${key}`, { value }),
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
