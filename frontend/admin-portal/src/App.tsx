import { ErrorBoundary } from './components/ErrorBoundary'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { AppLayout } from './components/layout/AppLayout'
import { LoginPage } from './pages/LoginPage'
import { DashboardPage } from './pages/DashboardPage'
import { AnomaliesPage } from './pages/AnomaliesPage'
import { AuditsPage } from './pages/AuditsPage'
import { NRWAnalysisPage } from './pages/NRWAnalysisPage'
import { FieldJobsPage } from './pages/FieldJobsPage'
import { MobileAppPage } from './pages/MobileAppPage'
import UserManagementPage from './pages/UserManagementPage'
import DistrictConfigPage from './pages/DistrictConfigPage'
import AuditThresholdsPage from './pages/AuditThresholdsPage'
import { GRACompliancePage } from './pages/GRACompliancePage'
import DMAMapPage from './pages/DMAMapPage'
import { ReportsPage } from './pages/ReportsPage'
import TariffManagementPage from './pages/TariffManagementPage'
import GapTrackingPage from './pages/GapTrackingPage'
import WhistleblowerPage from './pages/WhistleblowerPage'
import DonorKPIPage from './pages/DonorKPIPage'
import OfflineSyncStatusPage from './pages/OfflineSyncStatusPage'
import type { ReactNode } from 'react'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 30 * 1000,
    },
  },
})

function ProtectedRoute({ children }: { children: ReactNode }) {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="text-center">
          <div className="w-8 h-8 border-2 border-brand-500 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
          <p className="text-gray-500 text-sm">Loading GN-WAAS...</p>
        </div>
      </div>
    )
  }

  if (!user) {
    return <Navigate to="/login" replace />
  }

  return <>{children}</>
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/"
        element={
          <ProtectedRoute>
            <AppLayout />
          </ProtectedRoute>
        }
      >
        <Route index element={<Navigate to="/dashboard" replace />} />
        <Route path="dashboard" element={<DashboardPage />} />
        <Route path="anomalies" element={<AnomaliesPage />} />
        <Route path="audits" element={<AuditsPage />} />
        <Route path="nrw" element={<NRWAnalysisPage />} />
        <Route path="field-jobs" element={<FieldJobsPage />} />
        <Route path="dma-map" element={<DMAMapPage />} />
        <Route path="mobile-app" element={<MobileAppPage />} />
        <Route path="gra" element={<GRACompliancePage />} />
        <Route path="reports" element={<ReportsPage />} />
        <Route path="users" element={<UserManagementPage />} />
        <Route path="districts" element={<DistrictConfigPage />} />
        <Route path="settings" element={<AuditThresholdsPage />} />
        <Route path="tariffs" element={<TariffManagementPage />} />
        <Route path="gaps" element={<GapTrackingPage />} />
        <Route path="whistleblower" element={<WhistleblowerPage />} />
        <Route path="donor-kpis" element={<DonorKPIPage />} />
        <Route path="sync-status" element={<OfflineSyncStatusPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  )
}


export default function App() {
  return (
    <ErrorBoundary>
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <BrowserRouter basename={import.meta.env.BASE_URL}>
          <AppRoutes />
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
    </ErrorBoundary>
  )
}
