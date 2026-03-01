import React, { useState, useRef } from 'react'
import {
  View, Text, TouchableOpacity, StyleSheet, Alert,
  ActivityIndicator, ScrollView,
} from 'react-native'
import { CameraView, useCameraPermissions } from 'expo-camera'
import * as FileSystem from 'expo-file-system'
import * as Crypto from 'expo-crypto'
import * as Haptics from 'expo-haptics'
import { getCurrentPosition, isWithinFence } from '../../utils/gps'
import { useJobStore } from '../../store/jobStore'
import { apiClient } from '../../utils/api'
import type { MeterPhoto, OCRResult } from '../../types'

type CaptureStep = 'gps_check' | 'camera' | 'processing' | 'review' | 'notes' | 'submitting' | 'done'

export default function MeterCaptureScreen({ navigation }: { navigation: { goBack: () => void } }) {
  const [permission, requestPermission] = useCameraPermissions()
  const [step, setStep] = useState<CaptureStep>('gps_check')
  const [gpsStatus, setGpsStatus] = useState<'checking' | 'ok' | 'outside_fence' | 'error'>('checking')
  const [capturedPhoto, setCapturedPhoto] = useState<MeterPhoto | null>(null)
  const [ocrResult, setOcrResult] = useState<OCRResult | null>(null)
  const [manualReading, setManualReading] = useState('')
  const [notes, setNotes] = useState('')
  const cameraRef = useRef<CameraView>(null)

  const { activeJob, addPhoto, setOCRResult, setOfficerNotes } = useJobStore()

  // Step 1: GPS check
  const checkGPS = async () => {
    setGpsStatus('checking')
    try {
      const pos = await getCurrentPosition()
      if (!activeJob) { setGpsStatus('error'); return }

      const within = isWithinFence(pos.lat, pos.lng, activeJob.gps_lat, activeJob.gps_lng)
      if (within) {
        setGpsStatus('ok')
        await Haptics.notificationAsync(Haptics.NotificationFeedbackType.Success)
        setTimeout(() => setStep('camera'), 800)
      } else {
        setGpsStatus('outside_fence')
        await Haptics.notificationAsync(Haptics.NotificationFeedbackType.Error)
      }
    } catch (err: unknown) {
      setGpsStatus('error')
      Alert.alert('GPS Error', (err as Error).message)
    }
  }

  // Step 2: Capture photo
  const capturePhoto = async () => {
    if (!cameraRef.current) return
    try {
      await Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Medium)
      const photo = await cameraRef.current.takePictureAsync({ quality: 0.9, base64: false })
      if (!photo) return

      const pos = await getCurrentPosition()
      // Compute SHA-256 of the photo file for tamper-evidence
      let photoHash = 'sha256:unknown'
      try {
        const fileInfo = await FileSystem.readAsStringAsync(photo.uri, {
          encoding: FileSystem.EncodingType.Base64,
        })
        const digest = await Crypto.digestStringAsync(
          Crypto.CryptoDigestAlgorithm.SHA256,
          fileInfo,
        )
        photoHash = `sha256:${digest}`
      } catch (hashErr) {
        console.warn('Photo hash computation failed:', hashErr)
      }

      const meterPhoto: MeterPhoto = {
        uri: photo.uri,
        hash: photoHash,
        gps_lat: pos.lat,
        gps_lng: pos.lng,
        gps_accuracy: pos.accuracy,
        captured_at: new Date().toISOString(),
        within_fence: true,
      }

      addPhoto(meterPhoto)
      setCapturedPhoto(meterPhoto)
      setStep('processing')
      await processOCR(photo.uri, meterPhoto)
    } catch (err: unknown) {
      Alert.alert('Capture Error', (err as Error).message)
    }
  }

  // Step 3: OCR processing
  const processOCR = async (photoUri: string, photo: MeterPhoto) => {
    try {
      const formData = new FormData()
      formData.append('photo', { uri: photoUri, type: 'image/jpeg', name: 'meter.jpg' } as unknown as Blob)
      formData.append('gps_lat', photo.gps_lat.toString())
      formData.append('gps_lng', photo.gps_lng.toString())
      formData.append('meter_lat', activeJob?.gps_lat.toString() || '0')
      formData.append('meter_lng', activeJob?.gps_lng.toString() || '0')

      const res = await apiClient.post('/api/v1/ocr/process', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      })
      const result = res.data.data as OCRResult
      setOcrResult(result)
      setOCRResult(result)
    } catch {
      // OCR failed — fall back to manual entry
      setOcrResult({ reading_m3: 0, confidence: 0, status: 'FAILED', raw_text: '' })
    } finally {
      setStep('review')
    }
  }

  // Step 5: Submit
  const submitJob = async () => {
    if (!activeJob || !capturedPhoto) return
    setStep('submitting')
    try {
      const finalReading = ocrResult?.status === 'SUCCESS'
        ? ocrResult.reading_m3
        : parseFloat(manualReading)

      await apiClient.patch(`/api/v1/field-jobs/${activeJob.id}/complete`, {
        ocr_reading_m3: finalReading,
        ocr_confidence: ocrResult?.confidence || 0,
        ocr_status: ocrResult?.status || 'MANUAL',
        officer_notes: notes,
        gps_lat: capturedPhoto.gps_lat,
        gps_lng: capturedPhoto.gps_lng,
        gps_accuracy_m: capturedPhoto.gps_accuracy,
        photo_hashes: [capturedPhoto.hash],
      })

      await Haptics.notificationAsync(Haptics.NotificationFeedbackType.Success)
      setStep('done')
    } catch (err: unknown) {
      Alert.alert('Submission Error', (err as Error).message)
      setStep('review')
    }
  }

  if (!permission) return <View style={styles.center}><ActivityIndicator color="#2e7d32" /></View>
  if (!permission.granted) {
    return (
      <View style={styles.center}>
        <Text style={styles.permText}>Camera permission required</Text>
        <TouchableOpacity style={styles.btn} onPress={requestPermission}>
          <Text style={styles.btnText}>Grant Permission</Text>
        </TouchableOpacity>
      </View>
    )
  }

  return (
    <View style={styles.container}>
      {/* GPS Check */}
      {step === 'gps_check' && (
        <View style={styles.center}>
          <Text style={styles.stepTitle}>GPS Verification</Text>
          <Text style={styles.stepDesc}>
            Verifying you are within 50m of the meter location before capture is allowed.
          </Text>
          {gpsStatus === 'checking' && <ActivityIndicator size="large" color="#2e7d32" style={{ marginTop: 24 }} />}
          {gpsStatus === 'ok' && <Text style={styles.successText}>✅ Location verified — opening camera...</Text>}
          {gpsStatus === 'outside_fence' && (
            <View style={styles.errorBox}>
              <Text style={styles.errorText}>⚠️ You are outside the 50m fence for this meter.</Text>
              <Text style={styles.errorSub}>Move closer to the meter and try again.</Text>
              <TouchableOpacity style={styles.btn} onPress={checkGPS}><Text style={styles.btnText}>Retry GPS Check</Text></TouchableOpacity>
            </View>
          )}
          {gpsStatus === 'error' && (
            <TouchableOpacity style={styles.btn} onPress={checkGPS}><Text style={styles.btnText}>Retry</Text></TouchableOpacity>
          )}
          {gpsStatus === 'checking' && (
            <TouchableOpacity style={[styles.btn, { marginTop: 24 }]} onPress={checkGPS}>
              <Text style={styles.btnText}>Check GPS</Text>
            </TouchableOpacity>
          )}
        </View>
      )}

      {/* Camera */}
      {step === 'camera' && (
        <View style={styles.cameraContainer}>
          <CameraView ref={cameraRef} style={styles.camera} facing="back">
            <View style={styles.cameraOverlay}>
              <View style={styles.meterFrame} />
              <Text style={styles.cameraHint}>Align meter display within the frame</Text>
            </View>
          </CameraView>
          <TouchableOpacity style={styles.captureBtn} onPress={capturePhoto}>
            <View style={styles.captureBtnInner} />
          </TouchableOpacity>
        </View>
      )}

      {/* Processing */}
      {step === 'processing' && (
        <View style={styles.center}>
          <ActivityIndicator size="large" color="#2e7d32" />
          <Text style={styles.stepTitle} style={{ marginTop: 16 }}>Processing OCR...</Text>
          <Text style={styles.stepDesc}>Extracting meter reading from photo</Text>
        </View>
      )}

      {/* Review */}
      {step === 'review' && ocrResult && (
        <ScrollView style={styles.reviewContainer}>
          <Text style={styles.sectionTitle}>OCR Result</Text>
          <View style={[styles.ocrBox, { borderColor: ocrResult.status === 'SUCCESS' ? '#16a34a' : '#dc2626' }]}>
            {ocrResult.status === 'SUCCESS' ? (
              <>
                <Text style={styles.ocrReading}>{ocrResult.reading_m3} m³</Text>
                <Text style={styles.ocrConfidence}>Confidence: {(ocrResult.confidence * 100).toFixed(0)}%</Text>
              </>
            ) : (
              <Text style={styles.ocrFailed}>OCR failed — please enter reading manually</Text>
            )}
          </View>

          <Text style={styles.sectionTitle}>Officer Notes</Text>
          <View style={styles.notesBox}>
            <Text style={styles.notesPlaceholder}>Tap to add notes about the meter or property...</Text>
          </View>

          <TouchableOpacity style={styles.submitBtn} onPress={submitJob}>
            <Text style={styles.submitBtnText}>Submit Audit Evidence</Text>
          </TouchableOpacity>
        </ScrollView>
      )}

      {/* Submitting */}
      {step === 'submitting' && (
        <View style={styles.center}>
          <ActivityIndicator size="large" color="#2e7d32" />
          <Text style={{ marginTop: 16, color: '#374151' }}>Submitting to GN-WAAS...</Text>
        </View>
      )}

      {/* Done */}
      {step === 'done' && (
        <View style={styles.center}>
          <Text style={{ fontSize: 64 }}>✅</Text>
          <Text style={styles.stepTitle}>Audit Submitted</Text>
          <Text style={styles.stepDesc}>Evidence has been recorded and locked in the audit trail.</Text>
          <TouchableOpacity style={styles.btn} onPress={() => navigation.goBack()}>
            <Text style={styles.btnText}>Back to Jobs</Text>
          </TouchableOpacity>
        </View>
      )}
    </View>
  )
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#f9fafb' },
  center: { flex: 1, alignItems: 'center', justifyContent: 'center', padding: 24 },
  stepTitle: { fontSize: 20, fontWeight: '800', color: '#111827', textAlign: 'center', marginBottom: 8 },
  stepDesc: { fontSize: 14, color: '#6b7280', textAlign: 'center', lineHeight: 20 },
  permText: { fontSize: 16, color: '#374151', marginBottom: 16 },
  btn: { backgroundColor: '#2e7d32', borderRadius: 12, paddingVertical: 14, paddingHorizontal: 28, marginTop: 16 },
  btnText: { color: '#fff', fontWeight: '700', fontSize: 15 },
  successText: { fontSize: 16, color: '#16a34a', fontWeight: '600', marginTop: 24 },
  errorBox: { alignItems: 'center', marginTop: 24 },
  errorText: { fontSize: 15, color: '#dc2626', fontWeight: '600', textAlign: 'center' },
  errorSub: { fontSize: 13, color: '#6b7280', textAlign: 'center', marginTop: 8, marginBottom: 16 },
  cameraContainer: { flex: 1 },
  camera: { flex: 1 },
  cameraOverlay: { flex: 1, alignItems: 'center', justifyContent: 'center' },
  meterFrame: {
    width: 260, height: 120, borderWidth: 2, borderColor: '#fdd835',
    borderRadius: 8, backgroundColor: 'transparent',
  },
  cameraHint: { color: '#fff', fontSize: 13, marginTop: 12, backgroundColor: 'rgba(0,0,0,0.5)', paddingHorizontal: 12, paddingVertical: 6, borderRadius: 20 },
  captureBtn: {
    position: 'absolute', bottom: 40, alignSelf: 'center',
    width: 72, height: 72, borderRadius: 36, backgroundColor: '#fff',
    alignItems: 'center', justifyContent: 'center',
    shadowColor: '#000', shadowOffset: { width: 0, height: 4 }, shadowOpacity: 0.3, shadowRadius: 8, elevation: 8,
  },
  captureBtnInner: { width: 56, height: 56, borderRadius: 28, backgroundColor: '#2e7d32' },
  reviewContainer: { flex: 1, padding: 20 },
  sectionTitle: { fontSize: 14, fontWeight: '700', color: '#374151', marginBottom: 8, marginTop: 16 },
  ocrBox: { borderWidth: 2, borderRadius: 12, padding: 16, alignItems: 'center', backgroundColor: '#fff' },
  ocrReading: { fontSize: 36, fontWeight: '900', color: '#111827' },
  ocrConfidence: { fontSize: 13, color: '#6b7280', marginTop: 4 },
  ocrFailed: { fontSize: 14, color: '#dc2626', fontWeight: '600' },
  notesBox: { backgroundColor: '#fff', borderWidth: 1, borderColor: '#e5e7eb', borderRadius: 12, padding: 14, minHeight: 80 },
  notesPlaceholder: { color: '#9ca3af', fontSize: 14 },
  submitBtn: { backgroundColor: '#2e7d32', borderRadius: 14, paddingVertical: 16, alignItems: 'center', marginTop: 24, marginBottom: 40 },
  submitBtnText: { color: '#fff', fontSize: 16, fontWeight: '800' },
})
