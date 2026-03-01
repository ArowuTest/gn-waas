import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '../lib/api-client'
import {
  Smartphone, Shield, Camera, MapPin, RefreshCw,
  AlertTriangle, CheckCircle, Save, Info, Wifi
} from 'lucide-react'

// ─── Types ────────────────────────────────────────────────────────────────────

interface MobileConfig {
  geofence_radius_m: number
  require_biometric: boolean
  blind_audit_default: boolean
  require_surroundings_photo: boolean
  max_photo_age_minutes: number
  ocr_conflict_tolerance_pct: number
  sync_interval_seconds: number
  max_jobs_per_officer: number
  app_min_version: string
  app_latest_version: string
  force_update: boolean
  maintenance_mode: boolean
  maintenance_message: string
}

interface SystemConfig {
  id: string
  config_key: string
  config_value: string
  config_type: string
  description: string
  category: string
  updated_at: string
}

// ─── Config Section ───────────────────────────────────────────────────────────

function ConfigSection({
  title,
  icon,
  children,
}: {
  title: string
  icon: React.ReactNode
  children: React.ReactNode
}) {
  return (
    <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
      <div className="flex items-center gap-3 px-5 py-4 border-b border-gray-100 bg-gray-50">
        <span className="text-brand-600">{icon}</span>
        <h3 className="text-sm font-semibold text-gray-800">{title}</h3>
      </div>
      <div className="p-5 space-y-4">{children}</div>
    </div>
  )
}

function ConfigField({
  label,
  hint,
  children,
}: {
  label: string
  hint?: string
  children: React.ReactNode
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div className="flex-1">
        <p className="text-sm font-medium text-gray-700">{label}</p>
        {hint && <p className="text-xs text-gray-400 mt-0.5">{hint}</p>}
      </div>
      <div className="flex-shrink-0">{children}</div>
    </div>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export function MobileAppPage() {
  const queryClient = useQueryClient()
  const [saved, setSaved] = useState(false)

  // Fetch current mobile config from the API
  const { data: config, isLoading } = useQuery({
    queryKey: ['mobile-config'],
    queryFn: async () => {
      const res = await apiClient.get('/config/mobile')
      return (res.data.data ?? res.data) as MobileConfig
    },
  })

  // Fetch field.* system_config rows for editing
  const { data: fieldConfigs } = useQuery({
    queryKey: ['system-config', 'FIELD'],
    queryFn: async () => {
      const res = await apiClient.get('/admin/config?category=FIELD')
      return res.data.data as SystemConfig[]
    },
  })

  const [localConfig, setLocalConfig] = useState<Partial<MobileConfig>>({})

  const merged: MobileConfig = {
    geofence_radius_m:          100,
    require_biometric:          true,
    blind_audit_default:        true,
    require_surroundings_photo: true,
    max_photo_age_minutes:      5,
    ocr_conflict_tolerance_pct: 2.0,
    sync_interval_seconds:      30,
    max_jobs_per_officer:       5,
    app_min_version:            '1.0.0',
    app_latest_version:         '1.0.0',
    force_update:               false,
    maintenance_mode:           false,
    maintenance_message:        '',
    ...config,
    ...localConfig,
  }

  const saveMutation = useMutation({
    mutationFn: async (updates: Partial<MobileConfig>) => {
      // Map mobile config fields to system_config keys
      const keyMap: Record<string, string> = {
        geofence_radius_m:          'field.gps_fence_radius_m',
        require_biometric:          'field.require_biometric',
        blind_audit_default:        'field.blind_audit_default',
        require_surroundings_photo: 'field.require_surroundings_photo',
        max_photo_age_minutes:      'field.max_photo_age_minutes',
        ocr_conflict_tolerance_pct: 'field.ocr_conflict_tolerance_pct',
        sync_interval_seconds:      'field.sync_interval_seconds',
        max_jobs_per_officer:       'audit.max_jobs_per_officer',
        app_min_version:            'mobile.app_min_version',
        app_latest_version:         'mobile.app_latest_version',
        force_update:               'mobile.force_update',
        maintenance_mode:           'mobile.maintenance_mode',
        maintenance_message:        'mobile.maintenance_message',
      }
      await Promise.all(
        Object.entries(updates).map(([k, v]) => {
          const configKey = keyMap[k]
          if (!configKey) return Promise.resolve()
          return apiClient.patch('/admin/config', {
            config_key:   configKey,
            config_value: String(v),
          })
        })
      )
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mobile-config'] })
      queryClient.invalidateQueries({ queryKey: ['system-config'] })
      setLocalConfig({})
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    },
  })

  const update = <K extends keyof MobileConfig>(key: K, value: MobileConfig[K]) => {
    setLocalConfig(prev => ({ ...prev, [key]: value }))
  }

  const hasChanges = Object.keys(localConfig).length > 0

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="w-6 h-6 border-2 border-brand-500 border-t-transparent rounded-full animate-spin" />
      </div>
    )
  }

  return (
    <div className="space-y-6 max-w-3xl">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Mobile App Management</h1>
          <p className="text-sm text-gray-500 mt-0.5">
            Control field officer app behaviour. Changes take effect on next app sync (≤30s).
          </p>
        </div>
        <div className="flex items-center gap-3">
          {saved && (
            <span className="flex items-center gap-1.5 text-sm text-green-600 font-medium">
              <CheckCircle size={14} />
              Saved
            </span>
          )}
          <button
            onClick={() => saveMutation.mutate(localConfig)}
            disabled={!hasChanges || saveMutation.isPending}
            className="flex items-center gap-2 px-4 py-2 bg-brand-600 text-white rounded-lg text-sm font-medium hover:bg-brand-700 disabled:opacity-40"
          >
            <Save size={14} />
            {saveMutation.isPending ? 'Saving…' : 'Save Changes'}
          </button>
        </div>
      </div>

      {/* Maintenance Mode Banner */}
      {merged.maintenance_mode && (
        <div className="bg-orange-50 border border-orange-200 rounded-xl p-4 flex items-center gap-3">
          <AlertTriangle size={18} className="text-orange-500 flex-shrink-0" />
          <div>
            <p className="text-sm font-semibold text-orange-800">Maintenance Mode Active</p>
            <p className="text-xs text-orange-600">
              Field officers will see: "{merged.maintenance_message || 'System under maintenance'}"
            </p>
          </div>
        </div>
      )}

      {/* GPS & Location */}
      <ConfigSection title="GPS & Location" icon={<MapPin size={16} />}>
        <ConfigField
          label="Geofence Radius"
          hint="Maximum distance from meter GPS coordinates for evidence capture"
        >
          <div className="flex items-center gap-2">
            <input
              type="number"
              min={5}
              max={500}
              step={5}
              value={merged.geofence_radius_m}
              onChange={e => update('geofence_radius_m', Number(e.target.value))}
              className="w-24 text-sm border border-gray-200 rounded-lg px-3 py-1.5 text-right focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
            <span className="text-sm text-gray-500">metres</span>
          </div>
        </ConfigField>

        <ConfigField
          label="Blind Audit Mode"
          hint="Officers see GPS coordinates only — not customer name or account number"
        >
          <label className="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              checked={merged.blind_audit_default}
              onChange={e => update('blind_audit_default', e.target.checked)}
              className="sr-only peer"
            />
            <div className="w-10 h-5 bg-gray-200 peer-focus:ring-2 peer-focus:ring-brand-500 rounded-full peer peer-checked:bg-brand-600 after:content-[''] after:absolute after:top-0.5 after:left-0.5 after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:after:translate-x-5" />
          </label>
        </ConfigField>
      </ConfigSection>

      {/* Security */}
      <ConfigSection title="Security & Authentication" icon={<Shield size={16} />}>
        <ConfigField
          label="Require Biometric Login"
          hint="Field officers must use fingerprint/face ID after initial password login"
        >
          <label className="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              checked={merged.require_biometric}
              onChange={e => update('require_biometric', e.target.checked)}
              className="sr-only peer"
            />
            <div className="w-10 h-5 bg-gray-200 peer-focus:ring-2 peer-focus:ring-brand-500 rounded-full peer peer-checked:bg-brand-600 after:content-[''] after:absolute after:top-0.5 after:left-0.5 after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:after:translate-x-5" />
          </label>
        </ConfigField>
      </ConfigSection>

      {/* Photo & OCR */}
      <ConfigSection title="Photo & OCR Settings" icon={<Camera size={16} />}>
        <ConfigField
          label="Require Surroundings Photo"
          hint="Officers must capture a surroundings photo in addition to the meter face"
        >
          <label className="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              checked={merged.require_surroundings_photo}
              onChange={e => update('require_surroundings_photo', e.target.checked)}
              className="sr-only peer"
            />
            <div className="w-10 h-5 bg-gray-200 peer-focus:ring-2 peer-focus:ring-brand-500 rounded-full peer peer-checked:bg-brand-600 after:content-[''] after:absolute after:top-0.5 after:left-0.5 after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:after:translate-x-5" />
          </label>
        </ConfigField>

        <ConfigField
          label="Max Photo Age"
          hint="Reject meter photos older than this (prevents pre-captured photos)"
        >
          <div className="flex items-center gap-2">
            <input
              type="number"
              min={1}
              max={60}
              value={merged.max_photo_age_minutes}
              onChange={e => update('max_photo_age_minutes', Number(e.target.value))}
              className="w-20 text-sm border border-gray-200 rounded-lg px-3 py-1.5 text-right focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
            <span className="text-sm text-gray-500">minutes</span>
          </div>
        </ConfigField>

        <ConfigField
          label="OCR Conflict Tolerance"
          hint="Flag if OCR reading differs from manual entry by more than this %"
        >
          <div className="flex items-center gap-2">
            <input
              type="number"
              min={0.5}
              max={20}
              step={0.5}
              value={merged.ocr_conflict_tolerance_pct}
              onChange={e => update('ocr_conflict_tolerance_pct', Number(e.target.value))}
              className="w-20 text-sm border border-gray-200 rounded-lg px-3 py-1.5 text-right focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
            <span className="text-sm text-gray-500">%</span>
          </div>
        </ConfigField>
      </ConfigSection>

      {/* Sync & Capacity */}
      <ConfigSection title="Sync & Capacity" icon={<Wifi size={16} />}>
        <ConfigField
          label="Sync Interval"
          hint="How often the app syncs pending submissions when online"
        >
          <div className="flex items-center gap-2">
            <input
              type="number"
              min={10}
              max={300}
              step={10}
              value={merged.sync_interval_seconds}
              onChange={e => update('sync_interval_seconds', Number(e.target.value))}
              className="w-24 text-sm border border-gray-200 rounded-lg px-3 py-1.5 text-right focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
            <span className="text-sm text-gray-500">seconds</span>
          </div>
        </ConfigField>

        <ConfigField
          label="Max Jobs per Officer"
          hint="Maximum concurrent open jobs assigned to a single field officer"
        >
          <input
            type="number"
            min={1}
            max={20}
            value={merged.max_jobs_per_officer}
            onChange={e => update('max_jobs_per_officer', Number(e.target.value))}
            className="w-20 text-sm border border-gray-200 rounded-lg px-3 py-1.5 text-right focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </ConfigField>
      </ConfigSection>

      {/* App Version Control */}
      <ConfigSection title="App Version Control" icon={<Smartphone size={16} />}>
        <ConfigField
          label="Minimum Required Version"
          hint="Officers on older versions will be prompted to update"
        >
          <input
            type="text"
            value={merged.app_min_version}
            onChange={e => update('app_min_version', e.target.value)}
            placeholder="1.0.0"
            className="w-28 text-sm border border-gray-200 rounded-lg px-3 py-1.5 focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </ConfigField>

        <ConfigField
          label="Latest Version"
          hint="Displayed in the app's About screen"
        >
          <input
            type="text"
            value={merged.app_latest_version}
            onChange={e => update('app_latest_version', e.target.value)}
            placeholder="1.0.0"
            className="w-28 text-sm border border-gray-200 rounded-lg px-3 py-1.5 focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </ConfigField>

        <ConfigField
          label="Force Update"
          hint="Block app usage until officer updates to minimum version"
        >
          <label className="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              checked={merged.force_update}
              onChange={e => update('force_update', e.target.checked)}
              className="sr-only peer"
            />
            <div className="w-10 h-5 bg-gray-200 peer-focus:ring-2 peer-focus:ring-brand-500 rounded-full peer peer-checked:bg-red-500 after:content-[''] after:absolute after:top-0.5 after:left-0.5 after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:after:translate-x-5" />
          </label>
        </ConfigField>
      </ConfigSection>

      {/* Maintenance Mode */}
      <ConfigSection title="Maintenance Mode" icon={<AlertTriangle size={16} />}>
        <ConfigField
          label="Enable Maintenance Mode"
          hint="Disables field officer login and shows a maintenance message"
        >
          <label className="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              checked={merged.maintenance_mode}
              onChange={e => update('maintenance_mode', e.target.checked)}
              className="sr-only peer"
            />
            <div className="w-10 h-5 bg-gray-200 peer-focus:ring-2 peer-focus:ring-brand-500 rounded-full peer peer-checked:bg-orange-500 after:content-[''] after:absolute after:top-0.5 after:left-0.5 after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:after:translate-x-5" />
          </label>
        </ConfigField>

        <ConfigField
          label="Maintenance Message"
          hint="Shown to field officers when maintenance mode is active"
        >
          <input
            type="text"
            value={merged.maintenance_message}
            onChange={e => update('maintenance_message', e.target.value)}
            placeholder="System under maintenance. Please try again later."
            className="w-72 text-sm border border-gray-200 rounded-lg px-3 py-1.5 focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </ConfigField>
      </ConfigSection>

      {/* Info footer */}
      <div className="flex items-start gap-2 text-xs text-gray-400 pb-4">
        <Info size={12} className="mt-0.5 flex-shrink-0" />
        <p>
          Changes are written to the <code className="font-mono bg-gray-100 px-1 rounded">system_config</code> table
          and served via <code className="font-mono bg-gray-100 px-1 rounded">GET /api/v1/config/mobile</code>.
          The Flutter app fetches this endpoint on startup and caches it locally for offline use.
        </p>
      </div>
    </div>
  )
}
