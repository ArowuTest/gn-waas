import { ErrorBoundary } from './components/ErrorBoundary'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Layout } from './components/Layout';
import LoginPage from './pages/LoginPage';
import DashboardPage from './pages/DashboardPage';
import CaseQueuePage from './pages/CaseQueuePage';
import CaseDetailPage from './pages/CaseDetailPage';
import UnderbillingPage from './pages/UnderbillingPage';
import OverbillingPage from './pages/OverbillingPage';
import MisclassificationPage from './pages/MisclassificationPage';
import FieldAssignmentsPage from './pages/FieldAssignmentsPage';
import MonthlyReportPage from './pages/MonthlyReportPage';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 30_000,
    },
  },
});

function RequireAuth({ children }: { children: React.ReactNode }) {
  const token = localStorage.getItem('gwl_token');
  if (!token) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

export default function App() {
  return (
    <ErrorBoundary>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route
            path="/*"
            element={
              <RequireAuth>
                <Layout>
                  <Routes>
                    <Route path="/" element={<DashboardPage />} />
                    <Route path="/cases" element={<CaseQueuePage />} />
                    <Route path="/cases/:id" element={<CaseDetailPage />} />
                    <Route path="/underbilling" element={<UnderbillingPage />} />
                    <Route path="/overbilling" element={<OverbillingPage />} />
                    <Route path="/misclassification" element={<MisclassificationPage />} />
                    <Route path="/field-assignments" element={<FieldAssignmentsPage />} />
                    <Route path="/credits" element={<OverbillingPage />} />
                    <Route path="/reports" element={<MonthlyReportPage />} />
                    <Route path="*" element={<Navigate to="/" replace />} />
                  </Routes>
                </Layout>
              </RequireAuth>
            }
          />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
    </ErrorBoundary>
  );
}
