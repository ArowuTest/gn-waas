import React from 'react'
import {
  View, Text, FlatList, TouchableOpacity, StyleSheet,
  RefreshControl, ActivityIndicator,
} from 'react-native'
import { useQuery } from '@tanstack/react-query'
import { apiClient } from '../../utils/api'
import type { FieldJob, AlertLevel } from '../../types'

const alertColors: Record<AlertLevel, string> = {
  CRITICAL: '#dc2626',
  HIGH: '#ea580c',
  MEDIUM: '#d97706',
  LOW: '#2563eb',
  INFO: '#6b7280',
}

function JobCard({ job, onPress }: { job: FieldJob; onPress: () => void }) {
  const color = alertColors[job.alert_level]
  return (
    <TouchableOpacity style={styles.card} onPress={onPress} activeOpacity={0.7}>
      <View style={[styles.alertBar, { backgroundColor: color }]} />
      <View style={styles.cardContent}>
        <View style={styles.cardHeader}>
          <Text style={styles.customerName}>{job.customer_name}</Text>
          <View style={[styles.levelBadge, { backgroundColor: color + '20' }]}>
            <Text style={[styles.levelText, { color }]}>{job.alert_level}</Text>
          </View>
        </View>
        <Text style={styles.accountNum}>{job.account_number}</Text>
        <Text style={styles.address} numberOfLines={1}>📍 {job.address}</Text>
        <View style={styles.cardFooter}>
          <Text style={styles.anomalyType}>{job.anomaly_type.replace(/_/g, ' ')}</Text>
          {job.estimated_variance_ghs && (
            <Text style={styles.variance}>
              GHS {job.estimated_variance_ghs.toLocaleString()}
            </Text>
          )}
        </View>
      </View>
    </TouchableOpacity>
  )
}

export default function JobListScreen({ navigation }: { navigation: { navigate: (s: string, p?: object) => void } }) {
  const { data, isLoading, refetch, isRefetching } = useQuery({
    queryKey: ['my-jobs'],
    queryFn: async () => {
      const res = await apiClient.get('/api/v1/field-jobs/my-jobs')
      return res.data.data as FieldJob[]
    },
  })

  if (isLoading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator size="large" color="#2e7d32" />
        <Text style={styles.loadingText}>Loading your jobs...</Text>
      </View>
    )
  }

  const jobs = data || []
  const pending = jobs.filter(j => j.status !== 'COMPLETED' && j.status !== 'FAILED')
  const completed = jobs.filter(j => j.status === 'COMPLETED')

  return (
    <View style={styles.container}>
      {/* Header stats */}
      <View style={styles.statsRow}>
        <View style={styles.statBox}>
          <Text style={styles.statValue}>{pending.length}</Text>
          <Text style={styles.statLabel}>Pending</Text>
        </View>
        <View style={styles.statBox}>
          <Text style={[styles.statValue, { color: '#16a34a' }]}>{completed.length}</Text>
          <Text style={styles.statLabel}>Completed</Text>
        </View>
        <View style={styles.statBox}>
          <Text style={styles.statValue}>{jobs.length}</Text>
          <Text style={styles.statLabel}>Total Today</Text>
        </View>
      </View>

      <FlatList
        data={pending}
        keyExtractor={item => item.id}
        renderItem={({ item }) => (
          <JobCard
            job={item}
            onPress={() => navigation.navigate('JobDetail', { job: item })}
          />
        )}
        refreshControl={<RefreshControl refreshing={isRefetching} onRefresh={refetch} tintColor="#2e7d32" />}
        ListEmptyComponent={
          <View style={styles.empty}>
            <Text style={styles.emptyIcon}>✅</Text>
            <Text style={styles.emptyText}>All jobs completed for today!</Text>
          </View>
        }
        contentContainerStyle={styles.list}
      />
    </View>
  )
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#f9fafb' },
  center: { flex: 1, alignItems: 'center', justifyContent: 'center' },
  loadingText: { marginTop: 12, color: '#6b7280', fontSize: 14 },
  statsRow: {
    flexDirection: 'row', backgroundColor: '#2e7d32',
    paddingVertical: 16, paddingHorizontal: 20,
  },
  statBox: { flex: 1, alignItems: 'center' },
  statValue: { fontSize: 24, fontWeight: '900', color: '#fff' },
  statLabel: { fontSize: 11, color: '#a5d6a7', marginTop: 2 },
  list: { padding: 16, gap: 12 },
  card: {
    backgroundColor: '#fff', borderRadius: 14, flexDirection: 'row',
    overflow: 'hidden', shadowColor: '#000', shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.06, shadowRadius: 8, elevation: 3,
  },
  alertBar: { width: 5 },
  cardContent: { flex: 1, padding: 14 },
  cardHeader: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 },
  customerName: { fontSize: 15, fontWeight: '700', color: '#111827', flex: 1 },
  levelBadge: { paddingHorizontal: 8, paddingVertical: 3, borderRadius: 20 },
  levelText: { fontSize: 10, fontWeight: '700' },
  accountNum: { fontSize: 12, color: '#6b7280', marginBottom: 4 },
  address: { fontSize: 12, color: '#6b7280', marginBottom: 8 },
  cardFooter: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  anomalyType: { fontSize: 11, color: '#374151', backgroundColor: '#f3f4f6', paddingHorizontal: 8, paddingVertical: 3, borderRadius: 6 },
  variance: { fontSize: 13, fontWeight: '700', color: '#dc2626' },
  empty: { alignItems: 'center', paddingTop: 60 },
  emptyIcon: { fontSize: 48, marginBottom: 12 },
  emptyText: { fontSize: 16, color: '#6b7280', fontWeight: '500' },
})
