import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '../utils/api'

// ─── Types ────────────────────────────────────────────────────────────────────

interface District {
  id: string
  district_code: string
  district_name: string
  region: string
  population_estimate: number
  total_connections: number
  supply_status: string
  zone_type: string
  loss_ratio_pct?: number
  data_confidence_grade?: number
  is_pilot_district: boolean
  is_active: boolean
  manager_name?: string
  manager_email?: string
  created_at: string
}

const REGIONS = [
  'Greater Accra', 'Ashanti', 'Western', 'Central', 'Eastern',
  'Volta', 'Northern', 'Upper East', 'Upper West', 'Brong-Ahafo',
  'Oti', 'Bono East', 'Ahafo', 'Savannah', 'North East', 'Western North',
]

const SUPPLY_STATUSES = ['ACTIVE', 'INTERMITTENT', 'SUSPENDED', 'PLANNED']
const ZONE_TYPES = ['URBAN', 'PERI_URBAN', 'RURAL', 'INDUSTRIAL']

function gradeBadge(grade?: number) {
  if (!grade) return { label: 'N/A', cls: 'badge-gray' }
  if (grade >= 80) return { label: `A (${grade})`, cls: 'badge-green' }
  if (grade >= 60) return { label: `B (${grade})`, cls: 'badge-blue' }
  if (grade >= 40) return { label: `C (${grade})`, cls: 'badge-yellow' }
  return { label: `D (${grade})`, cls: 'badge-red' }
}

function lossColor(pct?: number) {
  if (!pct) return '#6b7280'
  if (pct > 50) return '#dc2626'
  if (pct > 35) return '#d97706'
  if (pct > 20) return '#ca8a04'
  return '#16a34a'
}

// ─── District Modal ───────────────────────────────────────────────────────────

function DistrictModal({
  district,
  onClose,
  onSave,
}: {
  district: District | null
  onClose: () => void
  onSave: (data: any) => void
}) {
  const [form, setForm] = useState({
    district_code: district?.district_code ?? '',
    district_name: district?.district_name ?? '',
    region: district?.region ?? 'Greater Accra',
    population_estimate: district?.population_estimate ?? 0,
    total_connections: district?.total_connections ?? 0,
    supply_status: district?.supply_status ?? 'ACTIVE',
    zone_type: district?.zone_type ?? 'URBAN',
    is_pilot_district: district?.is_pilot_district ?? false,
    is_active: district?.is_active ?? true,
  })

  const set = (k: string, v: any) => setForm(f => ({ ...f, [k]: v }))

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal modal--wide" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2>{district ? `Edit — ${district.district_name}` : 'Create District'}</h2>
          <button className="modal-close" onClick={onClose}>✕</button>
        </div>

        <div className="modal-body">
          <div className="form-grid form-grid--3">
            <div className="form-group">
              <label>District Code *</label>
              <input value={form.district_code} onChange={e => set('district_code', e.target.value)}
                placeholder="e.g. ACC-CENTRAL" disabled={!!district} />
            </div>
            <div className="form-group" style={{ gridColumn: 'span 2' }}>
              <label>District Name *</label>
              <input value={form.district_name} onChange={e => set('district_name', e.target.value)}
                placeholder="e.g. Accra Central District" />
            </div>
            <div className="form-group">
              <label>Region *</label>
              <select value={form.region} onChange={e => set('region', e.target.value)}>
                {REGIONS.map(r => <option key={r}>{r}</option>)}
              </select>
            </div>
            <div className="form-group">
              <label>Supply Status</label>
              <select value={form.supply_status} onChange={e => set('supply_status', e.target.value)}>
                {SUPPLY_STATUSES.map(s => <option key={s}>{s}</option>)}
              </select>
            </div>
            <div className="form-group">
              <label>Zone Type</label>
              <select value={form.zone_type} onChange={e => set('zone_type', e.target.value)}>
                {ZONE_TYPES.map(z => <option key={z}>{z}</option>)}
              </select>
            </div>
            <div className="form-group">
              <label>Population Estimate</label>
              <input type="number" value={form.population_estimate}
                onChange={e => set('population_estimate', parseInt(e.target.value) || 0)} />
            </div>
            <div className="form-group">
              <label>Total Connections</label>
              <input type="number" value={form.total_connections}
                onChange={e => set('total_connections', parseInt(e.target.value) || 0)} />
            </div>
            <div className="form-group form-group--checkboxes">
              <label className="checkbox-label">
                <input type="checkbox" checked={form.is_pilot_district}
                  onChange={e => set('is_pilot_district', e.target.checked)} />
                Pilot District
              </label>
              <label className="checkbox-label">
                <input type="checkbox" checked={form.is_active}
                  onChange={e => set('is_active', e.target.checked)} />
                Active
              </label>
            </div>
          </div>
        </div>

        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button
            className="btn btn-primary"
            onClick={() => onSave(form)}
            disabled={!form.district_code || !form.district_name}
          >
            {district ? 'Save Changes' : 'Create District'}
          </button>
        </div>
      </div>
    </div>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function DistrictConfigPage() {
  const queryClient = useQueryClient()
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState<District | null>(null)
  const [regionFilter, setRegionFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')

  const { data: districts = [], isLoading } = useQuery<District[]>({
    queryKey: ['admin-districts'],
    queryFn: async () => {
      const res = await apiClient.get('/api/v1/districts')
      return res.data.data ?? []
    },
  })

  const createDistrict = useMutation({
    mutationFn: (data: any) => apiClient.post('/api/v1/admin/districts', data),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['admin-districts'] }); setShowModal(false) },
  })

  const updateDistrict = useMutation({
    mutationFn: ({ id, ...data }: any) => apiClient.patch(`/api/v1/admin/districts/${id}`, data),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['admin-districts'] }); setShowModal(false) },
  })

  const handleSave = (data: any) => {
    if (editing) updateDistrict.mutate({ id: editing.id, ...data })
    else createDistrict.mutate(data)
  }

  const filtered = districts.filter(d => {
    if (regionFilter && d.region !== regionFilter) return false
    if (statusFilter && d.supply_status !== statusFilter) return false
    return true
  })

  const totalConnections = filtered.reduce((s, d) => s + d.total_connections, 0)
  const pilotCount = filtered.filter(d => d.is_pilot_district).length
  const avgLoss = filtered.filter(d => d.loss_ratio_pct).reduce((s, d, _, a) =>
    s + (d.loss_ratio_pct ?? 0) / a.length, 0)

  return (
    <div className="page">
      <style>{pageStyles}</style>

      {/* Header */}
      <div className="page-header">
        <div>
          <h1 className="page-title">District Configuration</h1>
          <p className="page-subtitle">Manage GWL service districts, DMA zones, and pilot assignments</p>
        </div>
        <button className="btn btn-primary" onClick={() => { setEditing(null); setShowModal(true) }}>
          + Add District
        </button>
      </div>

      {/* KPI strip */}
      <div className="kpi-strip">
        {[
          { label: 'Total Districts', value: filtered.length, icon: '🗺️' },
          { label: 'Pilot Districts', value: pilotCount, icon: '🧪' },
          { label: 'Total Connections', value: totalConnections.toLocaleString(), icon: '🔌' },
          { label: 'Avg NRW Loss', value: avgLoss ? `${avgLoss.toFixed(1)}%` : 'N/A', icon: '💧' },
        ].map(k => (
          <div key={k.label} className="kpi-card">
            <span className="kpi-icon">{k.icon}</span>
            <span className="kpi-value">{k.value}</span>
            <span className="kpi-label">{k.label}</span>
          </div>
        ))}
      </div>

      {/* Filters */}
      <div className="toolbar">
        <select className="filter-select" value={regionFilter} onChange={e => setRegionFilter(e.target.value)}>
          <option value="">All Regions</option>
          {REGIONS.map(r => <option key={r}>{r}</option>)}
        </select>
        <select className="filter-select" value={statusFilter} onChange={e => setStatusFilter(e.target.value)}>
          <option value="">All Statuses</option>
          {SUPPLY_STATUSES.map(s => <option key={s}>{s}</option>)}
        </select>
        <span className="result-count">{filtered.length} district(s)</span>
      </div>

      {/* Table */}
      {isLoading ? (
        <div className="loading">Loading districts...</div>
      ) : (
        <div className="table-wrapper">
          <table className="data-table">
            <thead>
              <tr>
                <th>Code</th>
                <th>District Name</th>
                <th>Region</th>
                <th>Zone</th>
                <th>Connections</th>
                <th>NRW Loss</th>
                <th>Data Grade</th>
                <th>Status</th>
                <th>Pilot</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map(d => {
                const grade = gradeBadge(d.data_confidence_grade)
                return (
                  <tr key={d.id}>
                    <td><code className="code-badge">{d.district_code}</code></td>
                    <td className="district-name">{d.district_name}</td>
                    <td className="text-sm text-gray">{d.region}</td>
                    <td><span className="zone-badge">{d.zone_type}</span></td>
                    <td className="text-right">{d.total_connections.toLocaleString()}</td>
                    <td>
                      {d.loss_ratio_pct != null ? (
                        <span className="loss-pct" style={{ color: lossColor(d.loss_ratio_pct) }}>
                          {d.loss_ratio_pct.toFixed(1)}%
                        </span>
                      ) : <span className="text-gray">—</span>}
                    </td>
                    <td><span className={`badge ${grade.cls}`}>{grade.label}</span></td>
                    <td>
                      <span className={`status-badge ${d.supply_status === 'ACTIVE' ? 'status-active' : 'status-warn'}`}>
                        {d.supply_status}
                      </span>
                    </td>
                    <td className="text-center">{d.is_pilot_district ? '🧪' : '—'}</td>
                    <td>
                      <button className="btn-icon" title="Edit"
                        onClick={() => { setEditing(d); setShowModal(true) }}>✏️</button>
                    </td>
                  </tr>
                )
              })}
              {filtered.length === 0 && (
                <tr><td colSpan={10} className="empty-row">No districts found</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {showModal && (
        <DistrictModal
          district={editing}
          onClose={() => setShowModal(false)}
          onSave={handleSave}
        />
      )}
    </div>
  )
}

// ─── Styles ───────────────────────────────────────────────────────────────────
const pageStyles = `
  .page { padding: 24px; max-width: 1400px; margin: 0 auto; }
  .page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 20px; }
  .page-title { font-size: 24px; font-weight: 700; color: #111827; margin: 0 0 4px; }
  .page-subtitle { font-size: 14px; color: #6b7280; margin: 0; }

  .kpi-strip { display: flex; gap: 12px; margin-bottom: 20px; }
  .kpi-card { flex: 1; background: #fff; border: 1px solid #e5e7eb; border-radius: 10px;
    padding: 14px 16px; display: flex; flex-direction: column; align-items: center; gap: 4px; }
  .kpi-icon { font-size: 20px; }
  .kpi-value { font-size: 22px; font-weight: 800; color: #111827; }
  .kpi-label { font-size: 11px; color: #6b7280; }

  .toolbar { display: flex; gap: 12px; align-items: center; margin-bottom: 16px; }
  .filter-select { padding: 8px 12px; border: 1px solid #d1d5db; border-radius: 8px; font-size: 14px; background: #fff; }
  .result-count { font-size: 13px; color: #6b7280; }

  .table-wrapper { background: #fff; border-radius: 12px; overflow: hidden; border: 1px solid #e5e7eb; }
  .data-table { width: 100%; border-collapse: collapse; }
  .data-table th { background: #f9fafb; padding: 11px 14px; text-align: left; font-size: 11px;
    font-weight: 600; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;
    border-bottom: 1px solid #e5e7eb; }
  .data-table td { padding: 12px 14px; border-bottom: 1px solid #f3f4f6; font-size: 13px; }
  .data-table tr:last-child td { border-bottom: none; }
  .data-table tr:hover td { background: #f9fafb; }
  .text-right { text-align: right; }
  .text-center { text-align: center; }
  .text-sm { font-size: 12px; }
  .text-gray { color: #9ca3af; }

  .code-badge { background: #f3f4f6; padding: 2px 6px; border-radius: 4px; font-size: 11px; font-family: monospace; }
  .district-name { font-weight: 600; color: #111827; }
  .zone-badge { background: #eff6ff; color: #1d4ed8; padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: 600; }
  .loss-pct { font-weight: 700; font-size: 13px; }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: 600; }
  .badge-green { background: #dcfce7; color: #166534; }
  .badge-blue { background: #dbeafe; color: #1d4ed8; }
  .badge-yellow { background: #fef9c3; color: #854d0e; }
  .badge-red { background: #fee2e2; color: #991b1b; }
  .badge-gray { background: #f3f4f6; color: #6b7280; }
  .status-badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: 600; }
  .status-active { background: #dcfce7; color: #166534; }
  .status-warn { background: #fef3c7; color: #92400e; }
  .empty-row { text-align: center; color: #9ca3af; padding: 40px !important; }
  .loading { text-align: center; padding: 40px; color: #6b7280; }

  .btn { padding: 8px 16px; border-radius: 8px; font-size: 14px; font-weight: 600; cursor: pointer; border: none; }
  .btn-primary { background: #2e7d32; color: #fff; }
  .btn-primary:hover { background: #1b5e20; }
  .btn-primary:disabled { background: #9ca3af; cursor: not-allowed; }
  .btn-secondary { background: #f3f4f6; color: #374151; }
  .btn-icon { width: 30px; height: 30px; border: none; border-radius: 6px; cursor: pointer;
    font-size: 14px; background: #f3f4f6; }
  .btn-icon:hover { background: #e5e7eb; }

  .modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5);
    display: flex; align-items: center; justify-content: center; z-index: 1000; }
  .modal { background: #fff; border-radius: 16px; width: 520px; max-width: 95vw;
    max-height: 90vh; overflow-y: auto; box-shadow: 0 20px 60px rgba(0,0,0,0.2); }
  .modal--wide { width: 680px; }
  .modal-header { display: flex; justify-content: space-between; align-items: center;
    padding: 20px 24px; border-bottom: 1px solid #e5e7eb; }
  .modal-header h2 { font-size: 18px; font-weight: 700; margin: 0; }
  .modal-close { background: none; border: none; font-size: 18px; cursor: pointer; color: #6b7280; }
  .modal-body { padding: 24px; }
  .modal-footer { display: flex; justify-content: flex-end; gap: 12px;
    padding: 16px 24px; border-top: 1px solid #e5e7eb; }

  .form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
  .form-grid--3 { grid-template-columns: 1fr 1fr 1fr; }
  .form-group { display: flex; flex-direction: column; gap: 6px; }
  .form-group label { font-size: 13px; font-weight: 600; color: #374151; }
  .form-group input, .form-group select { padding: 8px 12px; border: 1px solid #d1d5db;
    border-radius: 8px; font-size: 14px; outline: none; }
  .form-group input:focus, .form-group select:focus { border-color: #2e7d32; box-shadow: 0 0 0 3px rgba(46,125,50,0.1); }
  .form-group input:disabled { background: #f9fafb; color: #9ca3af; }
  .form-group--checkboxes { flex-direction: row; gap: 20px; align-items: center; padding-top: 20px; }
  .checkbox-label { display: flex; align-items: center; gap: 6px; font-size: 13px; font-weight: 600; color: #374151; cursor: pointer; }
`
