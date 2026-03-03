import axios from 'axios';

// V19-FE-01 fix: fallback to '/api/v1' (relative URL) so production Nginx proxy works
// without VITE_API_URL set. In dev, .env.development sets VITE_API_URL=http://localhost:3000
const BASE_URL = import.meta.env.VITE_API_URL
  ? `${import.meta.env.VITE_API_URL}/api/v1`
  : '/api/v1';

export const api = axios.create({
  baseURL: BASE_URL,
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' },
});

// Attach JWT token from localStorage on every request
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('gwl_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Handle 401 — redirect to login
api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('gwl_token');
      window.location.href = '/login';
    }
    return Promise.reject(err);
  }
);

// ─── GWL Case Management API ──────────────────────────────────────────────────

export const gwlApi = {
  // Dashboard summary
  getSummary: (districtId?: string) =>
    api.get('/gwl/cases/summary', { params: districtId ? { district_id: districtId } : {} }),

  // Case queue
  listCases: (params: Record<string, string | number | undefined>) =>
    api.get('/gwl/cases', { params }),

  getCase: (id: string) =>
    api.get(`/gwl/cases/${id}`),

  getCaseActions: (id: string) =>
    api.get(`/gwl/cases/${id}/actions`),

  // Case workflow
  assignFieldOfficer: (id: string, body: Record<string, unknown>) =>
    api.post(`/gwl/cases/${id}/assign`, body),

  updateStatus: (id: string, body: Record<string, unknown>) =>
    api.patch(`/gwl/cases/${id}/status`, body),

  requestReclassification: (id: string, body: Record<string, unknown>) =>
    api.post(`/gwl/cases/${id}/reclassify`, body),

  requestCredit: (id: string, body: Record<string, unknown>) =>
    api.post(`/gwl/cases/${id}/credit`, body),

  // Lists
  listReclassifications: (params?: Record<string, string>) =>
    api.get('/gwl/reclassifications', { params }),

  listCredits: (params?: Record<string, string>) =>
    api.get('/gwl/credits', { params }),

  // Reports
  getMonthlyReport: (period: string, districtId?: string) =>
    api.get('/gwl/reports/monthly', { params: { period, district_id: districtId } }),
};

// ─── Supporting APIs ──────────────────────────────────────────────────────────

export const districtApi = {
  list: () => api.get('/districts'),
};

export const userApi = {
  fieldOfficers: () => api.get('/users/field-officers'),
};
