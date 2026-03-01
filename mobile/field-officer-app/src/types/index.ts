// GN-WAAS Field Officer App — Type Definitions

export type FieldJobStatus = 'QUEUED' | 'DISPATCHED' | 'EN_ROUTE' | 'ON_SITE' | 'COMPLETED' | 'FAILED' | 'SOS'
export type OCRStatus = 'PENDING' | 'PROCESSING' | 'SUCCESS' | 'FAILED' | 'MANUAL'
export type AlertLevel = 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'INFO'

export interface User {
  id: string
  email: string
  full_name: string
  role: string
  badge_number?: string
  district_id?: string
}

export interface FieldJob {
  id: string
  audit_event_id: string
  account_number: string
  customer_name: string
  address: string
  gps_lat: number
  gps_lng: number
  anomaly_type: string
  alert_level: AlertLevel
  status: FieldJobStatus
  scheduled_at?: string
  dispatched_at?: string
  notes?: string
  estimated_variance_ghs?: number
}

export interface MeterPhoto {
  uri: string
  hash: string
  gps_lat: number
  gps_lng: number
  gps_accuracy: number
  captured_at: string
  within_fence: boolean
}

export interface OCRResult {
  reading_m3: number
  confidence: number
  status: OCRStatus
  raw_text: string
}

export interface JobSubmission {
  job_id: string
  ocr_reading_m3: number
  ocr_confidence: number
  ocr_status: OCRStatus
  officer_notes: string
  gps_lat: number
  gps_lng: number
  gps_accuracy_m: number
  photo_urls: string[]
  photo_hashes: string[]
}

export interface AuthState {
  user: User | null
  token: string | null
  isAuthenticated: boolean
}
