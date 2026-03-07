import React, { useState, useEffect } from 'react';

interface SyncStatus {
  pending: number;
  applied: number;
  conflicts: number;
  rejected: number;
}

interface SyncQueueItem {
  id: string;
  device_id: string;
  user_name: string;
  action_type: string;
  entity_type: string;
  entity_id: string;
  status: string;
  client_timestamp: string;
  processed_at?: string;
  created_at: string;
}

interface DeviceInfo {
  device_id: string;
  user_name: string;
  last_seen_at: string;
  pending_count: number;
  conflict_count: number;
}

const ACTION_LABELS: Record<string, string> = {
  FIELD_JOB_UPDATE: 'Job Status Update',
  GPS_CONFIRM: 'GPS Confirmation',
  METER_READING: 'Meter Reading',
  ANOMALY_REPORT: 'Anomaly Report',
};

const STATUS_COLORS: Record<string, string> = {
  PENDING: 'bg-yellow-100 text-yellow-800',
  APPLIED: 'bg-green-100 text-green-800',
  CONFLICT: 'bg-orange-100 text-orange-800',
  REJECTED: 'bg-red-100 text-red-800',
  SUPERSEDED: 'bg-gray-100 text-gray-600',
};

const OfflineSyncStatusPage: React.FC = () => {
  const [queueItems, setQueueItems] = useState<SyncQueueItem[]>([]);
  const [devices, setDevices] = useState<DeviceInfo[]>([]);
  const [globalStatus, setGlobalStatus] = useState<SyncStatus>({ pending: 0, applied: 0, conflicts: 0, rejected: 0 });
  const [loading, setLoading] = useState(true);
  const [statusFilter, setStatusFilter] = useState('');
  const [actionFilter, setActionFilter] = useState('');
  const [autoRefresh, setAutoRefresh] = useState(false);

  const apiBase = import.meta.env.VITE_API_URL || '';

  const fetchData = async () => {
    try {
      // Fetch sync queue (admin view of all users)
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);
      if (actionFilter) params.set('action_type', actionFilter);

      const [queueRes, devicesRes] = await Promise.all([
        fetch(`${apiBase}/api/v1/admin/sync/queue?${params}&limit=100`, {
          headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
        }),
        fetch(`${apiBase}/api/v1/admin/sync/devices`, {
          headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
        }),
      ]);

      if (queueRes.ok) {
        const data = await queueRes.json();
        const items: SyncQueueItem[] = data.items || [];
        setQueueItems(items);
        setGlobalStatus({
          pending: items.filter(i => i.status === 'PENDING').length,
          applied: items.filter(i => i.status === 'APPLIED').length,
          conflicts: items.filter(i => i.status === 'CONFLICT').length,
          rejected: items.filter(i => i.status === 'REJECTED').length,
        });
      }

      if (devicesRes.ok) {
        const data = await devicesRes.json();
        setDevices(data.devices || []);
      }
    } catch (e) {
      console.error('Failed to fetch sync data', e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, [statusFilter, actionFilter]);

  useEffect(() => {
    if (!autoRefresh) return;
    const interval = setInterval(fetchData, 10000);
    return () => clearInterval(interval);
  }, [autoRefresh, statusFilter, actionFilter]);

  const timeSince = (ts: string) => {
    const diff = Date.now() - new Date(ts).getTime();
    const mins = Math.floor(diff / 60000);
    if (mins < 1) return 'just now';
    if (mins < 60) return `${mins}m ago`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs}h ago`;
    return `${Math.floor(hrs / 24)}d ago`;
  };

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Offline Sync Status</h1>
          <p className="text-sm text-gray-500 mt-1">
            Field officer device sync queue · Ghana field operations
          </p>
        </div>
        <div className="flex items-center gap-3">
          <label className="flex items-center gap-2 text-sm text-gray-600">
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={e => setAutoRefresh(e.target.checked)}
              className="rounded"
            />
            Auto-refresh (10s)
          </label>
          <button
            onClick={fetchData}
            className="px-4 py-2 bg-blue-600 text-white rounded text-sm"
          >
            Refresh
          </button>
        </div>
      </div>

      {/* Global Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[
          { label: 'Pending', value: globalStatus.pending, color: 'text-yellow-600', bg: 'bg-yellow-50' },
          { label: 'Applied', value: globalStatus.applied, color: 'text-green-600', bg: 'bg-green-50' },
          { label: 'Conflicts', value: globalStatus.conflicts, color: 'text-orange-600', bg: 'bg-orange-50' },
          { label: 'Rejected', value: globalStatus.rejected, color: 'text-red-600', bg: 'bg-red-50' },
        ].map(s => (
          <div key={s.label} className={`${s.bg} rounded-lg border p-4`}>
            <p className="text-xs text-gray-500">{s.label}</p>
            <p className={`text-3xl font-bold ${s.color}`}>{s.value}</p>
          </div>
        ))}
      </div>

      {/* Active Devices */}
      {devices.length > 0 && (
        <div className="bg-white rounded-xl border p-6">
          <h2 className="text-lg font-bold text-gray-900 mb-4">Active Field Devices</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {devices.map(device => (
              <div key={device.device_id} className="border rounded-lg p-4">
                <div className="flex items-center justify-between mb-2">
                  <span className="font-medium text-sm">{device.user_name}</span>
                  <span className={`w-2 h-2 rounded-full ${
                    Date.now() - new Date(device.last_seen_at).getTime() < 300000
                      ? 'bg-green-500' : 'bg-gray-300'
                  }`} />
                </div>
                <p className="text-xs text-gray-500 font-mono truncate">{device.device_id}</p>
                <p className="text-xs text-gray-400 mt-1">Last seen: {timeSince(device.last_seen_at)}</p>
                <div className="flex gap-3 mt-2">
                  {device.pending_count > 0 && (
                    <span className="text-xs bg-yellow-100 text-yellow-700 px-2 py-0.5 rounded-full">
                      {device.pending_count} pending
                    </span>
                  )}
                  {device.conflict_count > 0 && (
                    <span className="text-xs bg-orange-100 text-orange-700 px-2 py-0.5 rounded-full">
                      {device.conflict_count} conflicts
                    </span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Filters */}
      <div className="flex gap-3 flex-wrap">
        <select
          value={statusFilter}
          onChange={e => setStatusFilter(e.target.value)}
          className="border rounded px-3 py-2 text-sm"
        >
          <option value="">All Statuses</option>
          {['PENDING','APPLIED','CONFLICT','REJECTED','SUPERSEDED'].map(s => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>
        <select
          value={actionFilter}
          onChange={e => setActionFilter(e.target.value)}
          className="border rounded px-3 py-2 text-sm"
        >
          <option value="">All Actions</option>
          {Object.entries(ACTION_LABELS).map(([k, v]) => (
            <option key={k} value={k}>{v}</option>
          ))}
        </select>
      </div>

      {/* Queue Table */}
      <div className="bg-white rounded-xl border overflow-hidden">
        <div className="px-6 py-4 border-b">
          <h2 className="text-lg font-bold text-gray-900">Sync Queue</h2>
          <p className="text-sm text-gray-500">Last 100 sync actions from all field devices</p>
        </div>
        {loading ? (
          <div className="p-8 text-center text-gray-500">Loading sync queue...</div>
        ) : queueItems.length === 0 ? (
          <div className="p-8 text-center text-gray-500">
            <div className="text-4xl mb-2">📱</div>
            <p>No sync actions found</p>
            <p className="text-xs mt-1">Field officers will appear here when they sync their devices</p>
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b">
              <tr>
                {['Officer','Device','Action','Entity','Status','Client Time','Processed','Queued'].map(h => (
                  <th key={h} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y">
              {queueItems.map(item => (
                <tr key={item.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 font-medium text-sm">{item.user_name}</td>
                  <td className="px-4 py-3 font-mono text-xs text-gray-500 max-w-24 truncate">{item.device_id}</td>
                  <td className="px-4 py-3">{ACTION_LABELS[item.action_type] || item.action_type}</td>
                  <td className="px-4 py-3 text-xs text-gray-500">{item.entity_type}</td>
                  <td className="px-4 py-3">
                    <span className={`px-2 py-1 rounded-full text-xs font-medium ${STATUS_COLORS[item.status] || 'bg-gray-100'}`}>
                      {item.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-xs text-gray-500">{timeSince(item.client_timestamp)}</td>
                  <td className="px-4 py-3 text-xs text-gray-500">
                    {item.processed_at ? timeSince(item.processed_at) : '—'}
                  </td>
                  <td className="px-4 py-3 text-xs text-gray-500">{timeSince(item.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Ghana Context Note */}
      <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
        <h3 className="text-sm font-bold text-blue-900 mb-1">📡 Ghana Field Operations Context</h3>
        <p className="text-xs text-blue-700">
          Field officers in Northern, Upper East/West, and rural Volta regions operate with intermittent 3G/4G coverage.
          The mobile app queues all actions locally (SQLite) and syncs automatically when connectivity is restored.
          Conflicts occur when the server data was updated between the officer's last pull and their push.
          Rejected actions indicate validation failures (e.g., GPS outside fence, invalid meter reading).
        </p>
      </div>
    </div>
  );
};

export default OfflineSyncStatusPage;
