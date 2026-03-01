import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { AppLayout } from './components/layout/AppLayout'
import { LoginPage } from './pages/LoginPage'
import { DashboardPage } from './pages/DashboardPage'
import { AnomaliesPage } from './pages/AnomaliesPage'
import { AuditsPage } from './pages/AuditsPage'
import { NRWAnalysisPage } from './pages/NRWAnalysisPage'
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
        {/* Placeholder routes for future pages */}
        <Route path="field-jobs" element={<PlaceholderPage title="Field Jobs" />} />
        <Route path="gra" element={<PlaceholderPage title="GRA Compliance" />} />
        <Route path="reports" element={<PlaceholderPage title="Reports" />} />
        <Route path="users" element={<UserManagementPage />} />
        <Route path="settings" element={<PlaceholderPage title="System Settings" />} />
      </Route>
      <Route path="*" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  )
}

function PlaceholderPage({ title }: { title: string }) {
  return (
    <div className="flex items-center justify-center h-64">
      <div className="text-center">
        <div className="text-4xl mb-3">🚧</div>
        <h2 className="text-gray-700">{title}</h2>
        <p className="text-gray-400 text-sm mt-1">This page is under construction</p>
      </div>
    </div>
  )
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <BrowserRouter>
          <AppRoutes />
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  )
}
