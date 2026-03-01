import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import AppLayout from './components/layout/AppLayout'
import LoginPage from './pages/LoginPage'
import MyDistrictPage from './pages/MyDistrictPage'
import AccountSearchPage from './pages/AccountSearchPage'
import MeterReadingPage from './pages/MeterReadingPage'
import ReportIssuePage from './pages/ReportIssuePage'
import MyJobsPage from './pages/MyJobsPage'
import NRWSummaryPage from './pages/NRWSummaryPage'
import JobAssignmentPage from './pages/JobAssignmentPage'
import ReportingPage from './pages/ReportingPage'

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 30_000, retry: 1 } },
})

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { user, isLoading } = useAuth()
  if (isLoading) return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="text-center">
        <div className="w-10 h-10 border-4 border-green-700 border-t-transparent rounded-full animate-spin mx-auto mb-3"></div>
        <p className="text-gray-500 text-sm">Loading GN-WAAS Authority Portal...</p>
      </div>
    </div>
  )
  return user ? <>{children}</> : <Navigate to="/login" replace />
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/" element={
              <ProtectedRoute>
                <AppLayout />
              </ProtectedRoute>
            }>
              <Route index element={<Navigate to="/district" replace />} />
              <Route path="district" element={<MyDistrictPage />} />
              <Route path="accounts" element={<AccountSearchPage />} />
              <Route path="meter-reading" element={<MeterReadingPage />} />
              <Route path="report-issue" element={<ReportIssuePage />} />
              <Route path="my-jobs" element={<MyJobsPage />} />
              <Route path="nrw" element={<NRWSummaryPage />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  )
}
