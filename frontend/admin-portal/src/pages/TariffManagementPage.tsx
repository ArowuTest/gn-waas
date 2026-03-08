import React, { useState, useEffect } from 'react';
import apiClient from '../lib/api-client';

// ── Types ─────────────────────────────────────────────────────────────────────

interface TariffRate {
  id: string;
  category: string;
  tier_name: string;
  min_volume_m3: number;
  max_volume_m3: number | null;
  rate_per_m3: number;
  service_charge_ghs: number;
  effective_from: string;
  effective_to: string | null;
  approved_by: string;
  regulatory_ref: string;
  is_active: boolean;
  created_at: string;
}

interface VATConfig {
  id: string;
  rate_percentage: number;
  effective_from: string;
  effective_to: string | null;
  regulatory_ref: string;
  is_active: boolean;
}

const CATEGORIES = ['RESIDENTIAL', 'COMMERCIAL', 'INDUSTRIAL', 'GOVERNMENT'];

// ── Component ─────────────────────────────────────────────────────────────────

export default function TariffManagementPage() {
  const [rates, setRates] = useState<TariffRate[]>([]);
  const [vatConfigs, setVatConfigs] = useState<VATConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'rates' | 'vat'>('rates');
  const [showAddRate, setShowAddRate] = useState(false);
  const [showAddVAT, setShowAddVAT] = useState(false);
  const [filterCategory, setFilterCategory] = useState('ALL');
  const [filterActive, setFilterActive] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // New rate form state
  const [newRate, setNewRate] = useState({
    category: 'RESIDENTIAL',
    tier_name: '',
    min_volume_m3: 0,
    max_volume_m3: '',
    rate_per_m3: '',
    service_charge_ghs: 0,
    effective_from: new Date().toISOString().split('T')[0],
    approved_by: '',
    regulatory_ref: '',
  });

  // New VAT form state
  const [newVAT, setNewVAT] = useState({
    rate_percentage: 20,
    effective_from: new Date().toISOString().split('T')[0],
    regulatory_ref: '',
  });

  useEffect(() => {
    fetchData();
  }, []);

  const fetchData = async () => {
    setLoading(true);
    try {
      const [ratesRes, vatRes] = await Promise.all([
        apiClient.get('/admin/tariffs'),
        apiClient.get('/admin/tariffs/vat'),
      ]);
      setRates(ratesRes.data.tariff_rates || []);
      setVatConfigs(vatRes.data.vat_configs || []);
    } catch (err: any) {
      setError('Failed to load tariff data: ' + (err.response?.data?.error || err.message));
    } finally {
      setLoading(false);
    }
  };

  const handleCreateRate = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await apiClient.post('/admin/tariffs', {
        ...newRate,
        min_volume_m3: parseFloat(String(newRate.min_volume_m3)),
        max_volume_m3: newRate.max_volume_m3 ? parseFloat(String(newRate.max_volume_m3)) : null,
        rate_per_m3: parseFloat(String(newRate.rate_per_m3)),
        service_charge_ghs: parseFloat(String(newRate.service_charge_ghs)),
      });
      setSuccess('Tariff rate created successfully');
      setShowAddRate(false);
      fetchData();
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to create tariff rate');
    } finally {
      setSaving(false);
    }
  };

  const handleDeactivate = async (id: string, tierName: string) => {
    if (!confirm(`Deactivate tariff rate "${tierName}"? This will stop it from being used for new bills.`)) return;
    try {
      await apiClient.patch(`/admin/tariffs/${id}/deactivate`);
      setSuccess(`Tariff rate "${tierName}" deactivated`);
      fetchData();
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to deactivate rate');
    }
  };

  const handleCreateVAT = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await apiClient.post('/admin/tariffs/vat', {
        rate_percentage: parseFloat(String(newVAT.rate_percentage)),
        effective_from: newVAT.effective_from,
        regulatory_ref: newVAT.regulatory_ref,
      });
      setSuccess('VAT configuration updated successfully');
      setShowAddVAT(false);
      fetchData();
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to update VAT config');
    } finally {
      setSaving(false);
    }
  };

  const filteredRates = rates.filter(r => {
    if (filterCategory !== 'ALL' && r.category !== filterCategory) return false;
    if (filterActive && !r.is_active) return false;
    return true;
  });

  const activeVAT = vatConfigs.find(v => v.is_active);

  return (
    <div className="p-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">PURC Tariff Management</h1>
          <p className="text-sm text-gray-500 mt-1">
            Manage water tariff rates and VAT configuration. Changes take effect immediately for new bills.
          </p>
        </div>
        <div className="flex gap-2">
          {activeTab === 'rates' && (
            <button
              onClick={() => setShowAddRate(true)}
              className="bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700"
            >
              + Add Tariff Rate
            </button>
          )}
          {activeTab === 'vat' && (
            <button
              onClick={() => setShowAddVAT(true)}
              className="bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700"
            >
              Update VAT Rate
            </button>
          )}
        </div>
      </div>

      {/* Alerts */}
      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-red-700 text-sm flex justify-between">
          <span>{error}</span>
          <button onClick={() => setError(null)} className="text-red-400 hover:text-red-600">✕</button>
        </div>
      )}
      {success && (
        <div className="mb-4 p-3 bg-green-50 border border-green-200 rounded-lg text-green-700 text-sm flex justify-between">
          <span>{success}</span>
          <button onClick={() => setSuccess(null)} className="text-green-400 hover:text-green-600">✕</button>
        </div>
      )}

      {/* Current VAT Banner */}
      {activeVAT && (
        <div className="mb-4 p-3 bg-blue-50 border border-blue-200 rounded-lg flex items-center gap-3">
          <span className="text-blue-600 font-semibold text-lg">{activeVAT.rate_percentage}% VAT</span>
          <span className="text-blue-700 text-sm">
            Active from {activeVAT.effective_from}
            {activeVAT.regulatory_ref && ` · ${activeVAT.regulatory_ref}`}
          </span>
        </div>
      )}

      {/* Tabs */}
      <div className="flex border-b border-gray-200 mb-4">
        {(['rates', 'vat'] as const).map(tab => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px ${
              activeTab === tab
                ? 'border-blue-600 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700'
            }`}
          >
            {tab === 'rates' ? 'Tariff Rates' : 'VAT Configuration'}
          </button>
        ))}
      </div>

      {/* Tariff Rates Tab */}
      {activeTab === 'rates' && (
        <>
          {/* Filters */}
          <div className="flex gap-3 mb-4">
            <select
              value={filterCategory}
              onChange={e => setFilterCategory(e.target.value)}
              className="border border-gray-300 rounded-lg px-3 py-1.5 text-sm"
            >
              <option value="ALL">All Categories</option>
              {CATEGORIES.map(c => <option key={c} value={c}>{c}</option>)}
            </select>
            <label className="flex items-center gap-2 text-sm text-gray-600">
              <input
                type="checkbox"
                checked={filterActive}
                onChange={e => setFilterActive(e.target.checked)}
                className="rounded"
              />
              Active only
            </label>
          </div>

          {loading ? (
            <div className="text-center py-12 text-gray-400">Loading tariff rates…</div>
          ) : (
            <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
              <table className="w-full text-sm">
                <thead className="bg-gray-50 border-b border-gray-200">
                  <tr>
                    <th className="text-left px-4 py-3 font-medium text-gray-600">Category</th>
                    <th className="text-left px-4 py-3 font-medium text-gray-600">Tier</th>
                    <th className="text-right px-4 py-3 font-medium text-gray-600">Volume (m³)</th>
                    <th className="text-right px-4 py-3 font-medium text-gray-600">Rate (GH₵/m³)</th>
                    <th className="text-right px-4 py-3 font-medium text-gray-600">Service Charge</th>
                    <th className="text-left px-4 py-3 font-medium text-gray-600">Effective From</th>
                    <th className="text-left px-4 py-3 font-medium text-gray-600">Ref</th>
                    <th className="text-center px-4 py-3 font-medium text-gray-600">Status</th>
                    <th className="px-4 py-3"></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {filteredRates.length === 0 ? (
                    <tr>
                      <td colSpan={9} className="text-center py-8 text-gray-400">
                        No tariff rates found. Add the current PURC schedule to get started.
                      </td>
                    </tr>
                  ) : filteredRates.map(rate => (
                    <tr key={rate.id} className={`hover:bg-gray-50 ${!rate.is_active ? 'opacity-50' : ''}`}>
                      <td className="px-4 py-3">
                        <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                          rate.category === 'RESIDENTIAL' ? 'bg-green-100 text-green-700' :
                          rate.category === 'COMMERCIAL' ? 'bg-blue-100 text-blue-700' :
                          rate.category === 'INDUSTRIAL' ? 'bg-orange-100 text-orange-700' :
                          'bg-purple-100 text-purple-700'
                        }`}>
                          {rate.category}
                        </span>
                      </td>
                      <td className="px-4 py-3 font-medium text-gray-900">{rate.tier_name}</td>
                      <td className="px-4 py-3 text-right text-gray-600">
                        {rate.min_volume_m3} – {rate.max_volume_m3 ?? '∞'}
                      </td>
                      <td className="px-4 py-3 text-right font-mono font-semibold text-gray-900">
                        {rate.rate_per_m3.toFixed(4)}
                      </td>
                      <td className="px-4 py-3 text-right text-gray-600">
                        {rate.service_charge_ghs > 0 ? `GH₵${rate.service_charge_ghs.toLocaleString()}` : '—'}
                      </td>
                      <td className="px-4 py-3 text-gray-600">{rate.effective_from}</td>
                      <td className="px-4 py-3 text-gray-500 text-xs">{rate.regulatory_ref || '—'}</td>
                      <td className="px-4 py-3 text-center">
                        {rate.is_active ? (
                          <span className="px-2 py-0.5 bg-green-100 text-green-700 rounded text-xs">Active</span>
                        ) : (
                          <span className="px-2 py-0.5 bg-gray-100 text-gray-500 rounded text-xs">
                            Ended {rate.effective_to}
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        {rate.is_active && (
                          <button
                            onClick={() => handleDeactivate(rate.id, rate.tier_name)}
                            className="text-red-500 hover:text-red-700 text-xs"
                          >
                            Deactivate
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      {/* VAT Config Tab */}
      {activeTab === 'vat' && (
        <div className="space-y-4">
          {vatConfigs.map(vat => (
            <div
              key={vat.id}
              className={`p-4 rounded-xl border ${vat.is_active ? 'border-blue-200 bg-blue-50' : 'border-gray-200 bg-white opacity-60'}`}
            >
              <div className="flex items-center justify-between">
                <div>
                  <span className="text-2xl font-bold text-gray-900">{vat.rate_percentage}%</span>
                  <span className="ml-2 text-gray-500 text-sm">VAT</span>
                  {vat.is_active && (
                    <span className="ml-2 px-2 py-0.5 bg-blue-600 text-white rounded text-xs">Current</span>
                  )}
                </div>
                <div className="text-right text-sm text-gray-500">
                  <div>Effective: {vat.effective_from}</div>
                  {vat.effective_to && <div>Ended: {vat.effective_to}</div>}
                  {vat.regulatory_ref && <div className="text-xs">{vat.regulatory_ref}</div>}
                </div>
              </div>
            </div>
          ))}
          {vatConfigs.length === 0 && (
            <div className="text-center py-8 text-gray-400">
              No VAT configuration found. Add the current GRA VAT rate.
            </div>
          )}
        </div>
      )}

      {/* Add Rate Modal */}
      {showAddRate && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-2xl shadow-xl w-full max-w-lg">
            <div className="p-6 border-b border-gray-200">
              <h2 className="text-lg font-semibold text-gray-900">Add PURC Tariff Rate</h2>
              <p className="text-sm text-gray-500 mt-1">
                Add a new tier from the current PURC tariff schedule.
              </p>
            </div>
            <form onSubmit={handleCreateRate} className="p-6 space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Category</label>
                  <select
                    value={newRate.category}
                    onChange={e => setNewRate({...newRate, category: e.target.value})}
                    className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm"
                  >
                    {CATEGORIES.map(c => <option key={c} value={c}>{c}</option>)}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Tier Name</label>
                  <input
                    type="text"
                    value={newRate.tier_name}
                    onChange={e => setNewRate({...newRate, tier_name: e.target.value})}
                    placeholder="e.g. Tier 1 (0–5 m³)"
                    className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm"
                    required
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Min Volume (m³)</label>
                  <input
                    type="number" step="0.01" min="0"
                    value={newRate.min_volume_m3}
                    onChange={e => setNewRate({...newRate, min_volume_m3: parseFloat(e.target.value)})}
                    className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm"
                    required
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Max Volume (m³, blank = unlimited)</label>
                  <input
                    type="number" step="0.01" min="0"
                    value={newRate.max_volume_m3}
                    onChange={e => setNewRate({...newRate, max_volume_m3: e.target.value})}
                    placeholder="Leave blank for unlimited"
                    className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Rate (GH₵/m³)</label>
                  <input
                    type="number" step="0.0001" min="0"
                    value={newRate.rate_per_m3}
                    onChange={e => setNewRate({...newRate, rate_per_m3: e.target.value})}
                    placeholder="e.g. 6.1225"
                    className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm"
                    required
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Service Charge (GH₵)</label>
                  <input
                    type="number" step="0.01" min="0"
                    value={newRate.service_charge_ghs}
                    onChange={e => setNewRate({...newRate, service_charge_ghs: parseFloat(e.target.value)})}
                    placeholder="0 for residential"
                    className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Effective From</label>
                  <input
                    type="date"
                    value={newRate.effective_from}
                    onChange={e => setNewRate({...newRate, effective_from: e.target.value})}
                    className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm"
                    required
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Approved By</label>
                  <input
                    type="text"
                    value={newRate.approved_by}
                    onChange={e => setNewRate({...newRate, approved_by: e.target.value})}
                    placeholder="e.g. PURC-2026-01"
                    className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm"
                  />
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Regulatory Reference</label>
                <input
                  type="text"
                  value={newRate.regulatory_ref}
                  onChange={e => setNewRate({...newRate, regulatory_ref: e.target.value})}
                  placeholder="e.g. PURC Notice 2026/01 — Water Tariff Review"
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm"
                />
              </div>
              <div className="flex gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setShowAddRate(false)}
                  className="flex-1 border border-gray-300 text-gray-700 px-4 py-2 rounded-lg text-sm"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={saving}
                  className="flex-1 bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
                >
                  {saving ? 'Saving…' : 'Create Rate'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Add VAT Modal */}
      {showAddVAT && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-2xl shadow-xl w-full max-w-md">
            <div className="p-6 border-b border-gray-200">
              <h2 className="text-lg font-semibold text-gray-900">Update VAT Rate</h2>
              <p className="text-sm text-gray-500 mt-1">
                This will deactivate the current VAT rate and activate the new one from the effective date.
              </p>
            </div>
            <form onSubmit={handleCreateVAT} className="p-6 space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">VAT Rate (%)</label>
                <input
                  type="number" step="0.01" min="0" max="100"
                  value={newVAT.rate_percentage}
                  onChange={e => setNewVAT({...newVAT, rate_percentage: parseFloat(e.target.value)})}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Effective From</label>
                <input
                  type="date"
                  value={newVAT.effective_from}
                  onChange={e => setNewVAT({...newVAT, effective_from: e.target.value})}
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Regulatory Reference</label>
                <input
                  type="text"
                  value={newVAT.regulatory_ref}
                  onChange={e => setNewVAT({...newVAT, regulatory_ref: e.target.value})}
                  placeholder="e.g. GRA Notice 2026/03"
                  className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm"
                />
              </div>
              <div className="flex gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setShowAddVAT(false)}
                  className="flex-1 border border-gray-300 text-gray-700 px-4 py-2 rounded-lg text-sm"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={saving}
                  className="flex-1 bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
                >
                  {saving ? 'Saving…' : 'Update VAT'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
