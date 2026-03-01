import React, { useState } from 'react'
import {
  View, Text, TouchableOpacity, StyleSheet, Alert,
  ActivityIndicator, Vibration,
} from 'react-native'
import * as Haptics from 'expo-haptics'
import { getCurrentPosition } from '../../utils/gps'
import { apiClient } from '../../utils/api'
import { useJobStore } from '../../store/jobStore'

export default function SOSScreen() {
  const [triggered, setTriggered] = useState(false)
  const [loading, setLoading] = useState(false)
  const { activeJob } = useJobStore()

  const triggerSOS = async () => {
    Alert.alert(
      '🚨 Trigger SOS?',
      'This will immediately alert your supervisor and dispatch emergency support to your GPS location.',
      [
        { text: 'Cancel', style: 'cancel' },
        {
          text: 'TRIGGER SOS',
          style: 'destructive',
          onPress: async () => {
            setLoading(true)
            try {
              const pos = await getCurrentPosition()
              await Haptics.notificationAsync(Haptics.NotificationFeedbackType.Error)
              Vibration.vibrate([0, 500, 200, 500, 200, 500])

              await apiClient.post(`/api/v1/field-jobs/${activeJob?.id || 'unknown'}/sos`, {
                gps_lat: pos.lat,
                gps_lng: pos.lng,
                gps_accuracy_m: pos.accuracy,
                notes: 'SOS triggered from mobile app',
              })

              setTriggered(true)
            } catch (err: unknown) {
              Alert.alert('SOS Error', 'Failed to send SOS. Please call your supervisor directly.')
            } finally {
              setLoading(false)
            }
          },
        },
      ]
    )
  }

  return (
    <View style={styles.container}>
      {triggered ? (
        <View style={styles.center}>
          <Text style={styles.sosIcon}>🚨</Text>
          <Text style={styles.triggeredTitle}>SOS Sent</Text>
          <Text style={styles.triggeredDesc}>
            Your supervisor has been alerted.{'\n'}
            Emergency support is being dispatched to your location.{'\n\n'}
            Stay calm and stay where you are.
          </Text>
          <View style={styles.infoBox}>
            <Text style={styles.infoLabel}>Supervisor Contact</Text>
            <Text style={styles.infoValue}>+233 20 000 0001</Text>
          </View>
          <View style={styles.infoBox}>
            <Text style={styles.infoLabel}>Emergency Services</Text>
            <Text style={styles.infoValue}>999 / 112</Text>
          </View>
        </View>
      ) : (
        <View style={styles.center}>
          <Text style={styles.title}>Emergency SOS</Text>
          <Text style={styles.desc}>
            Use this button if you feel unsafe, encounter a dangerous situation,
            or need immediate assistance in the field.
          </Text>

          <TouchableOpacity
            style={styles.sosButton}
            onPress={triggerSOS}
            disabled={loading}
            activeOpacity={0.8}
          >
            {loading ? (
              <ActivityIndicator size="large" color="#fff" />
            ) : (
              <>
                <Text style={styles.sosButtonText}>SOS</Text>
                <Text style={styles.sosButtonSub}>Hold to activate</Text>
              </>
            )}
          </TouchableOpacity>

          <Text style={styles.disclaimer}>
            SOS alerts are logged and reviewed.{'\n'}
            Only use in genuine emergencies.
          </Text>

          <View style={styles.contactsBox}>
            <Text style={styles.contactsTitle}>Direct Contacts</Text>
            <Text style={styles.contact}>Supervisor: +233 20 000 0001</Text>
            <Text style={styles.contact}>GN-WAAS Control: +233 30 000 0002</Text>
            <Text style={styles.contact}>Emergency: 999 / 112</Text>
          </View>
        </View>
      )}
    </View>
  )
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#fff5f5' },
  center: { flex: 1, alignItems: 'center', justifyContent: 'center', padding: 28 },
  title: { fontSize: 24, fontWeight: '900', color: '#111827', marginBottom: 12 },
  desc: { fontSize: 14, color: '#6b7280', textAlign: 'center', lineHeight: 22, marginBottom: 40 },
  sosButton: {
    width: 160, height: 160, borderRadius: 80,
    backgroundColor: '#dc2626', alignItems: 'center', justifyContent: 'center',
    shadowColor: '#dc2626', shadowOffset: { width: 0, height: 8 },
    shadowOpacity: 0.4, shadowRadius: 20, elevation: 12,
    borderWidth: 6, borderColor: '#fca5a5',
  },
  sosButtonText: { fontSize: 36, fontWeight: '900', color: '#fff' },
  sosButtonSub: { fontSize: 11, color: '#fca5a5', marginTop: 4 },
  disclaimer: { fontSize: 12, color: '#9ca3af', textAlign: 'center', marginTop: 32, lineHeight: 18 },
  contactsBox: {
    marginTop: 32, backgroundColor: '#fff', borderRadius: 14,
    padding: 16, width: '100%', borderWidth: 1, borderColor: '#fee2e2',
  },
  contactsTitle: { fontSize: 13, fontWeight: '700', color: '#374151', marginBottom: 8 },
  contact: { fontSize: 13, color: '#6b7280', marginBottom: 4 },
  sosIcon: { fontSize: 72, marginBottom: 16 },
  triggeredTitle: { fontSize: 28, fontWeight: '900', color: '#dc2626', marginBottom: 12 },
  triggeredDesc: { fontSize: 14, color: '#374151', textAlign: 'center', lineHeight: 22, marginBottom: 24 },
  infoBox: {
    backgroundColor: '#fff', borderRadius: 12, padding: 14, width: '100%',
    marginBottom: 8, borderWidth: 1, borderColor: '#fee2e2',
    flexDirection: 'row', justifyContent: 'space-between',
  },
  infoLabel: { fontSize: 13, color: '#6b7280' },
  infoValue: { fontSize: 13, fontWeight: '700', color: '#dc2626' },
})
