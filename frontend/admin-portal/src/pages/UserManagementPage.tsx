import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '../utils/api'

// ─── Types ────────────────────────────────────────────────────────────────────

interface SystemUser {
  id: string
  email: string
  full_name: string
  role: string
  district_id?: string
  district_name?: string
  badge_number?: string
  is_active: boolean
  last_login_at?: string
  created_at: string
}

interface District {
  id: string
  name: string
  code: string
}

const ROLES = [
  { value: 'SYSTEM_ADMIN', label: 'System Admin', color: '#7c3aed' },
  { value: 'AUDIT_MANAGER', label: 'Audit Manager', color: '#1d4ed8' },
  { value: 'DISTRICT_MANAGER', label: 'District Manager', color: '#0369a1' },
  { value: 'FIELD_OFFICER', label: 'Field Officer', color: '#047857' },
  { value: 'GRA_LIAISON', label: 'GRA Liaison', color: '#b45309' },
  { value: 'READONLY_VIEWER', label: 'Read-Only Viewer', color: '#6b7280' },
]

function roleBadge(role: string) {
  const r = ROLES.find(x => x.value === role)
  return r ? { label: r.label, color: r.color } : { label: role, color: '#6b7280' }
}

// ─── Create/Edit User Modal ───────────────────────────────────────────────────

function UserModal({
  user,
  districts,
  onClose,
  onSave,
}: {
  user: SystemUser | null
  districts: District[]
  onClose: () => void
  onSave: (data: Partial<SystemUser> & { password?: string }) => void
}) {
  const [form, setForm] = useState({
    full_name: user?.full_name ?? '',
    email: user?.email ?? '',
    role: user?.role ?? 'FIELD_OFFICER',
    district_id: user?.district_id ?? '',
    badge_number: user?.badge_number ?? '',
    password: '',
    is_active: user?.is_active ?? true,
  })

  const needsDistrict = ['DISTRICT_MANAGER', 'FIELD_OFFICER'].includes(form.role)

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2>{user ? 'Edit User' : 'Create User'}</h2>
          <button className="modal-close" onClick={onClose}>✕</button>
        </div>

        <div className="modal-body">
          <div className="form-grid">
            <div className="form-group">
              <label>Full Name *</label>
              <input
                value={form.full_name}
                onChange={e => setForm(f => ({ ...f, full_name: e.target.value }))}
                placeholder="e.g. Kwame Mensah"
              />
            </div>
            <div className="form-group">
              <label>Email *</label>
              <input
                type="email"
                value={form.email}
                onChange={e => setForm(f => ({ ...f, email: e.target.value }))}
                placeholder="kwame@gwl.gov.gh"
                disabled={!!user}
              />
            </div>
            <div className="form-group">
              <label>Role *</label>
              <select
                value={form.role}
                onChange={e => setForm(f => ({ ...f, role: e.target.value }))}
              >
                {ROLES.map(r => (
                  <option key={r.value} value={r.value}>{r.label}</option>
                ))}
              </select>
            </div>
            {needsDistrict && (
              <div className="form-group">
                <label>District *</label>
                <select
                  value={form.district_id}
                  onChange={e => setForm(f => ({ ...f, district_id: e.target.value }))}
                >
                  <option value="">— Select District —</option>
                  {districts.map(d => (
                    <option key={d.id} value={d.id}>{d.name}</option>
                  ))}
                </select>
              </div>
            )}
            {form.role === 'FIELD_OFFICER' && (
              <div className="form-group">
                <label>Badge Number</label>
                <input
                  value={form.badge_number}
                  onChange={e => setForm(f => ({ ...f, badge_number: e.target.value }))}
                  placeholder="e.g. FO-2026-001"
                />
              </div>
            )}
            {!user && (
              <div className="form-group">
                <label>Temporary Password *</label>
                <input
                  type="password"
                  value={form.password}
                  onChange={e => setForm(f => ({ ...f, password: e.target.value }))}
                  placeholder="Min 8 characters"
                />
              </div>
            )}
            {user && (
              <div className="form-group form-group--checkbox">
                <label>
                  <input
                    type="checkbox"
                    checked={form.is_active}
                    onChange={e => setForm(f => ({ ...f, is_active: e.target.checked }))}
                  />
                  &nbsp; Account Active
                </label>
              </div>
            )}
          </div>
        </div>

        <div className="modal-footer">
          <button className="btn btn-secondary" onClick={onClose}>Cancel</button>
          <button
            className="btn btn-primary"
            onClick={() => onSave(form)}
            disabled={!form.full_name || !form.email || (!user && !form.password)}
          >
            {user ? 'Save Changes' : 'Create User'}
          </button>
        </div>
      </div>
    </div>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function UserManagementPage() {
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [roleFilter, setRoleFilter] = useState('')
  const [editingUser, setEditingUser] = useState<SystemUser | null | 'new'>('new' as any)
  const [showModal, setShowModal] = useState(false)

  const { data: users = [], isLoading } = useQuery<SystemUser[]>({
    queryKey: ['admin-users', search, roleFilter],
    queryFn: async () => {
      const params = new URLSearchParams()
      if (search) params.set('q', search)
      if (roleFilter) params.set('role', roleFilter)
      const res = await apiClient.get(`/admin/users?${params}`)
      return res.data.data ?? []
    },
  })

  const { data: districts = [] } = useQuery<District[]>({
    queryKey: ['districts'],
    queryFn: async () => {
      const res = await apiClient.get('/districts')
      return res.data.data ?? []
    },
  })

  const createUser = useMutation({
    mutationFn: (data: any) => apiClient.post('/admin/users', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] })
      setShowModal(false)
    },
  })

  const updateUser = useMutation({
    mutationFn: ({ id, ...data }: any) => apiClient.patch(`/admin/users/${id}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] })
      setShowModal(false)
    },
  })

  const deactivateUser = useMutation({
    mutationFn: (id: string) => apiClient.patch(`/admin/users/${id}`, { is_active: false }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin-users'] }),
  })

  const resetPassword = useMutation({
    mutationFn: (id: string) => apiClient.post(`/admin/users/${id}/reset-password`),
    onSuccess: () => alert('Password reset email sent'),
  })

  const handleSave = (data: any) => {
    if (editingUser && editingUser !== 'new' && (editingUser as SystemUser).id) {
      updateUser.mutate({ id: (editingUser as SystemUser).id, ...data })
    } else {
      createUser.mutate(data)
    }
  }

  const roleCounts = ROLES.map(r => ({
    ...r,
    count: users.filter(u => u.role === r.value).length,
  }))

  return (
    <div className="page">
      <style>{pageStyles}</style>

      {/* Header */}
      <div className="page-header">
        <div>
          <h1 className="page-title">User Management</h1>
          <p className="page-subtitle">
            Manage GN-WAAS system users, roles, and district assignments
          </p>
        </div>
        <button
          className="btn btn-primary"
          onClick={() => { setEditingUser(null); setShowModal(true) }}
        >
          + Create User
        </button>
      </div>

      {/* Role summary cards */}
      <div className="role-cards">
        {roleCounts.map(r => (
          <div
            key={r.value}
            className={`role-card ${roleFilter === r.value ? 'role-card--active' : ''}`}
            onClick={() => setRoleFilter(roleFilter === r.value ? '' : r.value)}
            style={{ borderColor: r.color }}
          >
            <span className="role-card-count" style={{ color: r.color }}>{r.count}</span>
            <span className="role-card-label">{r.label}</span>
          </div>
        ))}
      </div>

      {/* Search + filter bar */}
      <div className="toolbar">
        <input
          className="search-input"
          placeholder="Search by name or email..."
          value={search}
          onChange={e => setSearch(e.target.value)}
        />
        <select
          className="filter-select"
          value={roleFilter}
          onChange={e => setRoleFilter(e.target.value)}
        >
          <option value="">All Roles</option>
          {ROLES.map(r => <option key={r.value} value={r.value}>{r.label}</option>)}
        </select>
        <span className="result-count">{users.length} user(s)</span>
      </div>

      {/* Users table */}
      {isLoading ? (
        <div className="loading">Loading users...</div>
      ) : (
        <div className="table-wrapper">
          <table className="data-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Email</th>
                <th>Role</th>
                <th>District</th>
                <th>Status</th>
                <th>Last Login</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {users.map(user => {
                const badge = roleBadge(user.role)
                return (
                  <tr key={user.id}>
                    <td>
                      <div className="user-name">{user.full_name}</div>
                      {user.badge_number && (
                        <div className="user-badge">🪪 {user.badge_number}</div>
                      )}
                    </td>
                    <td className="email-cell">{user.email}</td>
                    <td>
                      <span
                        className="role-badge"
                        style={{ backgroundColor: badge.color + '18', color: badge.color }}
                      >
                        {badge.label}
                      </span>
                    </td>
                    <td>{user.district_name ?? '—'}</td>
                    <td>
                      <span className={`status-badge ${user.is_active ? 'status-active' : 'status-inactive'}`}>
                        {user.is_active ? 'Active' : 'Inactive'}
                      </span>
                    </td>
                    <td className="date-cell">
                      {user.last_login_at
                        ? new Date(user.last_login_at).toLocaleDateString('en-GH')
                        : 'Never'}
                    </td>
                    <td>
                      <div className="action-buttons">
                        <button
                          className="btn-icon btn-icon--edit"
                          title="Edit"
                          onClick={() => { setEditingUser(user); setShowModal(true) }}
                        >✏️</button>
                        <button
                          className="btn-icon btn-icon--reset"
                          title="Reset Password"
                          onClick={() => resetPassword.mutate(user.id)}
                        >🔑</button>
                        {user.is_active && (
                          <button
                            className="btn-icon btn-icon--deactivate"
                            title="Deactivate"
                            onClick={() => {
                              if (confirm(`Deactivate ${user.full_name}?`)) {
                                deactivateUser.mutate(user.id)
                              }
                            }}
                          >🚫</button>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
              {users.length === 0 && (
                <tr>
                  <td colSpan={7} className="empty-row">No users found</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Create/Edit Modal */}
      {showModal && (
        <UserModal
          user={editingUser as SystemUser | null}
          districts={districts}
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
  .page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 24px; }
  .page-title { font-size: 24px; font-weight: 700; color: #111827; margin: 0 0 4px; }
  .page-subtitle { font-size: 14px; color: #6b7280; margin: 0; }

  .role-cards { display: flex; gap: 12px; margin-bottom: 20px; flex-wrap: wrap; }
  .role-card {
    flex: 1; min-width: 120px; padding: 12px 16px; background: #fff;
    border: 2px solid #e5e7eb; border-radius: 10px; cursor: pointer;
    display: flex; flex-direction: column; align-items: center; gap: 4px;
    transition: all 0.15s;
  }
  .role-card:hover { transform: translateY(-1px); box-shadow: 0 4px 12px rgba(0,0,0,0.08); }
  .role-card--active { background: #f0fdf4; }
  .role-card-count { font-size: 22px; font-weight: 800; }
  .role-card-label { font-size: 11px; color: #6b7280; text-align: center; }

  .toolbar { display: flex; gap: 12px; align-items: center; margin-bottom: 16px; }
  .search-input {
    flex: 1; padding: 8px 14px; border: 1px solid #d1d5db; border-radius: 8px;
    font-size: 14px; outline: none;
  }
  .search-input:focus { border-color: #2e7d32; box-shadow: 0 0 0 3px rgba(46,125,50,0.1); }
  .filter-select {
    padding: 8px 12px; border: 1px solid #d1d5db; border-radius: 8px;
    font-size: 14px; background: #fff; cursor: pointer;
  }
  .result-count { font-size: 13px; color: #6b7280; white-space: nowrap; }

  .table-wrapper { background: #fff; border-radius: 12px; overflow: hidden; border: 1px solid #e5e7eb; }
  .data-table { width: 100%; border-collapse: collapse; }
  .data-table th {
    background: #f9fafb; padding: 12px 16px; text-align: left;
    font-size: 12px; font-weight: 600; color: #6b7280; text-transform: uppercase;
    letter-spacing: 0.05em; border-bottom: 1px solid #e5e7eb;
  }
  .data-table td { padding: 14px 16px; border-bottom: 1px solid #f3f4f6; font-size: 14px; }
  .data-table tr:last-child td { border-bottom: none; }
  .data-table tr:hover td { background: #f9fafb; }

  .user-name { font-weight: 600; color: #111827; }
  .user-badge { font-size: 11px; color: #6b7280; margin-top: 2px; }
  .email-cell { color: #374151; font-size: 13px; }
  .date-cell { color: #6b7280; font-size: 13px; }

  .role-badge {
    display: inline-block; padding: 3px 10px; border-radius: 20px;
    font-size: 11px; font-weight: 600;
  }
  .status-badge {
    display: inline-block; padding: 3px 10px; border-radius: 20px;
    font-size: 11px; font-weight: 600;
  }
  .status-active { background: #dcfce7; color: #166534; }
  .status-inactive { background: #fee2e2; color: #991b1b; }

  .action-buttons { display: flex; gap: 6px; }
  .btn-icon {
    width: 30px; height: 30px; border: none; border-radius: 6px;
    cursor: pointer; font-size: 14px; display: flex; align-items: center;
    justify-content: center; background: #f3f4f6; transition: background 0.15s;
  }
  .btn-icon:hover { background: #e5e7eb; }

  .empty-row { text-align: center; color: #9ca3af; padding: 40px !important; }
  .loading { text-align: center; padding: 40px; color: #6b7280; }

  .btn { padding: 8px 16px; border-radius: 8px; font-size: 14px; font-weight: 600; cursor: pointer; border: none; }
  .btn-primary { background: #2e7d32; color: #fff; }
  .btn-primary:hover { background: #1b5e20; }
  .btn-primary:disabled { background: #9ca3af; cursor: not-allowed; }
  .btn-secondary { background: #f3f4f6; color: #374151; }
  .btn-secondary:hover { background: #e5e7eb; }

  .modal-overlay {
    position: fixed; inset: 0; background: rgba(0,0,0,0.5);
    display: flex; align-items: center; justify-content: center; z-index: 1000;
  }
  .modal {
    background: #fff; border-radius: 16px; width: 560px; max-width: 95vw;
    max-height: 90vh; overflow-y: auto; box-shadow: 0 20px 60px rgba(0,0,0,0.2);
  }
  .modal-header {
    display: flex; justify-content: space-between; align-items: center;
    padding: 20px 24px; border-bottom: 1px solid #e5e7eb;
  }
  .modal-header h2 { font-size: 18px; font-weight: 700; margin: 0; }
  .modal-close { background: none; border: none; font-size: 18px; cursor: pointer; color: #6b7280; }
  .modal-body { padding: 24px; }
  .modal-footer {
    display: flex; justify-content: flex-end; gap: 12px;
    padding: 16px 24px; border-top: 1px solid #e5e7eb;
  }

  .form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
  .form-group { display: flex; flex-direction: column; gap: 6px; }
  .form-group label { font-size: 13px; font-weight: 600; color: #374151; }
  .form-group input, .form-group select {
    padding: 8px 12px; border: 1px solid #d1d5db; border-radius: 8px;
    font-size: 14px; outline: none;
  }
  .form-group input:focus, .form-group select:focus {
    border-color: #2e7d32; box-shadow: 0 0 0 3px rgba(46,125,50,0.1);
  }
  .form-group input:disabled { background: #f9fafb; color: #9ca3af; }
  .form-group--checkbox { flex-direction: row; align-items: center; grid-column: span 2; }
`
