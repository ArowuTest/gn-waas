import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { gwlApi, districtApi, userApi } from '../utils/api';

export const QUERY_KEYS = {
  summary: (districtId?: string) => ['gwl', 'summary', districtId],
  cases: (params: Record<string, unknown>) => ['gwl', 'cases', params],
  case: (id: string) => ['gwl', 'case', id],
  caseActions: (id: string) => ['gwl', 'case', id, 'actions'],
  reclassifications: (params?: Record<string, string>) => ['gwl', 'reclassifications', params],
  credits: (params?: Record<string, string>) => ['gwl', 'credits', params],
  monthlyReport: (period: string, districtId?: string) => ['gwl', 'report', period, districtId],
  districts: () => ['districts'],
  fieldOfficers: () => ['field-officers'],
};

// ── Dashboard ─────────────────────────────────────────────────────────────────
export function useCaseSummary(districtId?: string) {
  return useQuery({
    queryKey: QUERY_KEYS.summary(districtId),
    queryFn: () => gwlApi.getSummary(districtId).then((r) => r.data.data),
    refetchInterval: 60_000,
  });
}

// ── Case Queue ────────────────────────────────────────────────────────────────
export function useCases(params: Record<string, string | number | undefined>) {
  return useQuery({
    queryKey: QUERY_KEYS.cases(params),
    queryFn: () => gwlApi.listCases(params).then((r) => r.data.data),
    placeholderData: (prev) => prev,
  });
}

export function useCase(id: string) {
  return useQuery({
    queryKey: QUERY_KEYS.case(id),
    queryFn: () => gwlApi.getCase(id).then((r) => r.data.data),
    enabled: !!id,
  });
}

export function useCaseActions(id: string) {
  return useQuery({
    queryKey: QUERY_KEYS.caseActions(id),
    queryFn: () => gwlApi.getCaseActions(id).then((r) => r.data.data),
    enabled: !!id,
  });
}

// ── Mutations ─────────────────────────────────────────────────────────────────
export function useAssignFieldOfficer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: Record<string, unknown> }) =>
      gwlApi.assignFieldOfficer(id, body),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: ['gwl', 'case', id] });
      qc.invalidateQueries({ queryKey: ['gwl', 'cases'] });
      qc.invalidateQueries({ queryKey: ['gwl', 'summary'] });
    },
  });
}

export function useUpdateCaseStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: Record<string, unknown> }) =>
      gwlApi.updateStatus(id, body),
    onSuccess: (_, { id }) => {
      qc.invalidateQueries({ queryKey: ['gwl', 'case', id] });
      qc.invalidateQueries({ queryKey: ['gwl', 'cases'] });
      qc.invalidateQueries({ queryKey: ['gwl', 'summary'] });
    },
  });
}

export function useRequestReclassification() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: Record<string, unknown> }) =>
      gwlApi.requestReclassification(id, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['gwl'] });
    },
  });
}

export function useRequestCredit() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: Record<string, unknown> }) =>
      gwlApi.requestCredit(id, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['gwl'] });
    },
  });
}

// ── Reclassifications & Credits ───────────────────────────────────────────────
export function useReclassifications(params?: Record<string, string>) {
  return useQuery({
    queryKey: QUERY_KEYS.reclassifications(params),
    queryFn: () => gwlApi.listReclassifications(params).then((r) => r.data.data),
  });
}

export function useCredits(params?: Record<string, string>) {
  return useQuery({
    queryKey: QUERY_KEYS.credits(params),
    queryFn: () => gwlApi.listCredits(params).then((r) => r.data.data),
  });
}

// ── Monthly Report ────────────────────────────────────────────────────────────
export function useMonthlyReport(period: string, districtId?: string) {
  return useQuery({
    queryKey: QUERY_KEYS.monthlyReport(period, districtId),
    queryFn: () => gwlApi.getMonthlyReport(period, districtId).then((r) => r.data.data),
    enabled: !!period,
  });
}

// ── Supporting data ───────────────────────────────────────────────────────────
export function useDistricts() {
  return useQuery({
    queryKey: QUERY_KEYS.districts(),
    queryFn: () => districtApi.list().then((r) => r.data.data),
    staleTime: 5 * 60_000,
  });
}

export function useFieldOfficers() {
  return useQuery({
    queryKey: QUERY_KEYS.fieldOfficers(),
    queryFn: () => userApi.fieldOfficers().then((r) => r.data.data),
    staleTime: 5 * 60_000,
  });
}
