import React, { useState, useEffect } from 'react';

interface Tip {
  id: string;
  tip_reference: string;
  category: string;
  status: string;
  district_name?: string;
  gwl_account_number?: string;
  description: string;
  reward_eligible: boolean;
  reward_amount_ghs?: number;
  linked_audit_event_id?: string;
  created_at: string;
  updated_at: string;
}

const CATEGORY_LABELS: Record<string, string> = {
  GHOST_ACCOUNT: 'Ghost Account',
  PHANTOM_METER: 'Phantom Meter',
  BILLING_MANIPULATION: 'Billing Manipulation',
  METER_TAMPERING: 'Meter Tampering',
  COLLUSION: 'Staff Collusion',
  ILLEGAL_CONNECTION: 'Illegal Connection',
  FIELD_OFFICER_FRAUD: 'Field Officer Fraud',
  OTHER: 'Other',
};

const STATUS_COLORS: Record<string, string> = {
  NEW: 'bg-blue-100 text-blue-800',
  UNDER_REVIEW: 'bg-yellow-100 text-yellow-800',
  INVESTIGATING: 'bg-orange-100 text-orange-800',
  CONFIRMED: 'bg-red-100 text-red-800',
  DISMISSED: 'bg-gray-100 text-gray-600',
  REWARDED: 'bg-green-100 text-green-800',
};

const WhistleblowerPage: React.FC = () => {
  const [tips, setTips] = useState<Tip[]>([]);
  const [loading, setLoading] = useState(true);
  const [statusFilter, setStatusFilter] = useState('');
  const [categoryFilter, setCategoryFilter] = useState('');
  const [selectedTip, setSelectedTip] = useState<Tip | null>(null);
  const [updateForm, setUpdateForm] = useState({
    status: '',
    investigation_notes: '',
    linked_audit_event_id: '',
    reward_amount_ghs: '',
  });
  const [saving, setSaving] = useState(false);
  const [stats, setStats] = useState({
    total: 0, new: 0, investigating: 0, confirmed: 0, rewards_ghs: 0,
  });

  const apiBase = import.meta.env.VITE_API_URL || '';

  const fetchTips = async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);
      if (categoryFilter) params.set('category', categoryFilter);
      const res = await fetch(`${apiBase}/api/v1/admin/tips?${params}`, {
        headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
      });
      const data = await res.json();
      const tipList: Tip[] = data.tips || [];
      setTips(tipList);
      setStats({
        total: tipList.length,
        new: tipList.filter(t => t.status === 'NEW').length,
        investigating: tipList.filter(t => t.status === 'INVESTIGATING').length,
        confirmed: tipList.filter(t => t.status === 'CONFIRMED').length,
        rewards_ghs: tipList.reduce((s, t) => s + (t.reward_amount_ghs || 0), 0),
      });
    } catch (e) {
      console.error('Failed to fetch tips', e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchTips(); }, [statusFilter, categoryFilter]);

  const handleUpdate = async () => {
    if (!selectedTip) return;
    setSaving(true);
    try {
      const body: Record<string, unknown> = {};
      if (updateForm.status) body.status = updateForm.status;
      if (updateForm.investigation_notes) body.investigation_notes = updateForm.investigation_notes;
      if (updateForm.linked_audit_event_id) body.linked_audit_event_id = updateForm.linked_audit_event_id;
      if (updateForm.reward_amount_ghs) body.reward_amount_ghs = parseFloat(updateForm.reward_amount_ghs);

      await fetch(`${apiBase}/api/v1/admin/tips/${selectedTip.id}`, {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${localStorage.getItem('token')}`,
        },
        body: JSON.stringify(body),
      });
      setSelectedTip(null);
      fetchTips();
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Whistleblower Tips</h1>
        <p className="text-sm text-gray-500 mt-1">
          Anonymous fraud reports from customers, staff, and community members
        </p>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
        {[
          { label: 'Total Tips', value: stats.total, color: 'text-gray-900' },
          { label: 'New', value: stats.new, color: 'text-blue-600' },
          { label: 'Investigating', value: stats.investigating, color: 'text-orange-600' },
          { label: 'Confirmed Fraud', value: stats.confirmed, color: 'text-red-600' },
          { label: 'Rewards Issued', value: `GH₵${stats.rewards_ghs.toFixed(2)}`, color: 'text-green-600' },
        ].map(s => (
          <div key={s.label} className="bg-white rounded-lg border p-4">
            <p className="text-xs text-gray-500">{s.label}</p>
            <p className={`text-xl font-bold ${s.color}`}>{s.value}</p>
          </div>
        ))}
      </div>

      {/* Filters */}
      <div className="flex gap-3 flex-wrap">
        <select
          value={statusFilter}
          onChange={e => setStatusFilter(e.target.value)}
          className="border rounded px-3 py-2 text-sm"
        >
          <option value="">All Statuses</option>
          {['NEW','UNDER_REVIEW','INVESTIGATING','CONFIRMED','DISMISSED','REWARDED'].map(s => (
            <option key={s} value={s}>{s.replace('_', ' ')}</option>
          ))}
        </select>
        <select
          value={categoryFilter}
          onChange={e => setCategoryFilter(e.target.value)}
          className="border rounded px-3 py-2 text-sm"
        >
          <option value="">All Categories</option>
          {Object.entries(CATEGORY_LABELS).map(([k, v]) => (
            <option key={k} value={k}>{v}</option>
          ))}
        </select>
      </div>

      {/* Tips Table */}
      <div className="bg-white rounded-lg border overflow-hidden">
        {loading ? (
          <div className="p-8 text-center text-gray-500">Loading tips...</div>
        ) : tips.length === 0 ? (
          <div className="p-8 text-center text-gray-500">No tips found</div>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b">
              <tr>
                {['Reference','Category','Status','District','Account','Reward','Submitted','Actions'].map(h => (
                  <th key={h} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y">
              {tips.map(tip => (
                <tr key={tip.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 font-mono text-xs font-medium text-blue-600">{tip.tip_reference}</td>
                  <td className="px-4 py-3">{CATEGORY_LABELS[tip.category] || tip.category}</td>
                  <td className="px-4 py-3">
                    <span className={`px-2 py-1 rounded-full text-xs font-medium ${STATUS_COLORS[tip.status] || 'bg-gray-100'}`}>
                      {tip.status.replace('_', ' ')}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-gray-600">{tip.district_name || '—'}</td>
                  <td className="px-4 py-3 font-mono text-xs">{tip.gwl_account_number || '—'}</td>
                  <td className="px-4 py-3">
                    {tip.reward_eligible ? (
                      <span className="text-green-600 font-medium">
                        {tip.reward_amount_ghs ? `GH₵${tip.reward_amount_ghs.toFixed(2)}` : 'Eligible'}
                      </span>
                    ) : (
                      <span className="text-gray-400">No</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-gray-500 text-xs">
                    {new Date(tip.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3">
                    <button
                      onClick={() => {
                        setSelectedTip(tip);
                        setUpdateForm({ status: tip.status, investigation_notes: '', linked_audit_event_id: tip.linked_audit_event_id || '', reward_amount_ghs: tip.reward_amount_ghs?.toString() || '' });
                      }}
                      className="text-blue-600 hover:underline text-xs"
                    >
                      Investigate
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Investigation Modal */}
      {selectedTip && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-xl shadow-xl w-full max-w-2xl">
            <div className="p-6 border-b">
              <h2 className="text-lg font-bold">Investigate Tip: {selectedTip.tip_reference}</h2>
              <p className="text-sm text-gray-500 mt-1">
                Category: {CATEGORY_LABELS[selectedTip.category]} | Submitted: {new Date(selectedTip.created_at).toLocaleDateString()}
              </p>
            </div>
            <div className="p-6 space-y-4">
              {/* Description */}
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Tip Description</label>
                <div className="bg-gray-50 rounded p-3 text-sm text-gray-800 whitespace-pre-wrap">
                  {selectedTip.description}
                </div>
              </div>

              {/* Status */}
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Update Status</label>
                <select
                  value={updateForm.status}
                  onChange={e => setUpdateForm(f => ({ ...f, status: e.target.value }))}
                  className="w-full border rounded px-3 py-2 text-sm"
                >
                  {['NEW','UNDER_REVIEW','INVESTIGATING','CONFIRMED','DISMISSED','REWARDED'].map(s => (
                    <option key={s} value={s}>{s.replace('_', ' ')}</option>
                  ))}
                </select>
              </div>

              {/* Notes */}
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Investigation Notes</label>
                <textarea
                  value={updateForm.investigation_notes}
                  onChange={e => setUpdateForm(f => ({ ...f, investigation_notes: e.target.value }))}
                  rows={3}
                  className="w-full border rounded px-3 py-2 text-sm"
                  placeholder="Internal investigation notes..."
                />
              </div>

              {/* Linked Audit */}
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Linked Audit Event ID</label>
                <input
                  type="text"
                  value={updateForm.linked_audit_event_id}
                  onChange={e => setUpdateForm(f => ({ ...f, linked_audit_event_id: e.target.value }))}
                  className="w-full border rounded px-3 py-2 text-sm font-mono"
                  placeholder="UUID of linked audit event"
                />
              </div>

              {/* Reward */}
              {selectedTip.reward_eligible && (
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Reward Amount (GH₵)</label>
                  <input
                    type="number"
                    value={updateForm.reward_amount_ghs}
                    onChange={e => setUpdateForm(f => ({ ...f, reward_amount_ghs: e.target.value }))}
                    className="w-full border rounded px-3 py-2 text-sm"
                    placeholder="3% of recovered amount"
                  />
                </div>
              )}
            </div>
            <div className="p-6 border-t flex justify-end gap-3">
              <button
                onClick={() => setSelectedTip(null)}
                className="px-4 py-2 border rounded text-sm"
              >
                Cancel
              </button>
              <button
                onClick={handleUpdate}
                disabled={saving}
                className="px-4 py-2 bg-blue-600 text-white rounded text-sm disabled:opacity-50"
              >
                {saving ? 'Saving...' : 'Save Investigation'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default WhistleblowerPage;
