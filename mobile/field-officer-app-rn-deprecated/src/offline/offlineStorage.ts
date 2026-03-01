/**
 * GN-WAAS Field Officer App — Offline Storage
 *
 * Implements offline-first capability using expo-sqlite for local persistence.
 * When the device has no network, jobs and evidence are stored locally.
 * A background sync process uploads pending evidence when connectivity returns.
 *
 * Architecture:
 *   1. Jobs are fetched from API and cached in SQLite
 *   2. Evidence (photos, OCR results, GPS) is stored locally first
 *   3. Background sync uploads pending evidence when online
 *   4. Conflict resolution: server wins for job status, local wins for evidence
 */

import * as SQLite from 'expo-sqlite'
import NetInfo from '@react-native-community/netinfo'
import { apiClient } from '../utils/api'
import { FieldJob, JobSubmission } from '../types'

const DB_NAME = 'gnwaas_offline.db'
const SCHEMA_VERSION = 2

// ─── Database Initialization ──────────────────────────────────────────────────

let db: SQLite.SQLiteDatabase | null = null

export async function initOfflineDB(): Promise<SQLite.SQLiteDatabase> {
  if (db) return db

  db = await SQLite.openDatabaseAsync(DB_NAME)

  await db.execAsync(`
    PRAGMA journal_mode = WAL;
    PRAGMA foreign_keys = ON;

    CREATE TABLE IF NOT EXISTS schema_version (
      version INTEGER PRIMARY KEY
    );

    CREATE TABLE IF NOT EXISTS offline_jobs (
      id                TEXT PRIMARY KEY,
      job_reference     TEXT NOT NULL,
      audit_event_id    TEXT,
      account_number    TEXT NOT NULL,
      customer_name     TEXT NOT NULL,
      address           TEXT NOT NULL,
      gps_lat           REAL NOT NULL,
      gps_lng           REAL NOT NULL,
      anomaly_type      TEXT,
      alert_level       TEXT NOT NULL DEFAULT 'MEDIUM',
      status            TEXT NOT NULL DEFAULT 'QUEUED',
      priority          INTEGER NOT NULL DEFAULT 2,
      scheduled_at      TEXT,
      dispatched_at     TEXT,
      notes             TEXT,
      estimated_variance_ghs REAL,
      synced_at         TEXT,
      created_at        TEXT NOT NULL DEFAULT (datetime('now')),
      updated_at        TEXT NOT NULL DEFAULT (datetime('now'))
    );

    CREATE TABLE IF NOT EXISTS pending_submissions (
      id                TEXT PRIMARY KEY,
      job_id            TEXT NOT NULL REFERENCES offline_jobs(id),
      submission_json   TEXT NOT NULL,
      photo_uris        TEXT NOT NULL DEFAULT '[]',
      status            TEXT NOT NULL DEFAULT 'PENDING',
      retry_count       INTEGER NOT NULL DEFAULT 0,
      last_error        TEXT,
      created_at        TEXT NOT NULL DEFAULT (datetime('now')),
      attempted_at      TEXT
    );

    CREATE TABLE IF NOT EXISTS offline_photos (
      id                TEXT PRIMARY KEY,
      job_id            TEXT NOT NULL REFERENCES offline_jobs(id),
      submission_id     TEXT REFERENCES pending_submissions(id),
      local_uri         TEXT NOT NULL,
      photo_hash        TEXT NOT NULL,
      gps_lat           REAL NOT NULL,
      gps_lng           REAL NOT NULL,
      gps_accuracy      REAL NOT NULL,
      captured_at       TEXT NOT NULL,
      within_fence      INTEGER NOT NULL DEFAULT 0,
      uploaded          INTEGER NOT NULL DEFAULT 0,
      remote_url        TEXT,
      created_at        TEXT NOT NULL DEFAULT (datetime('now'))
    );

    CREATE INDEX IF NOT EXISTS idx_pending_status ON pending_submissions(status);
    CREATE INDEX IF NOT EXISTS idx_photos_job ON offline_photos(job_id);
    CREATE INDEX IF NOT EXISTS idx_jobs_status ON offline_jobs(status);
  `)

  // Check/update schema version
  const versionRow = await db.getFirstAsync<{ version: number }>(
    'SELECT version FROM schema_version LIMIT 1'
  )
  if (!versionRow) {
    await db.runAsync('INSERT INTO schema_version (version) VALUES (?)', [SCHEMA_VERSION])
  }

  return db
}

// ─── Job Cache ────────────────────────────────────────────────────────────────

/**
 * Cache jobs fetched from the API into SQLite for offline access
 */
export async function cacheJobs(jobs: FieldJob[]): Promise<void> {
  const database = await initOfflineDB()

  await database.withTransactionAsync(async () => {
    for (const job of jobs) {
      await database.runAsync(
        `INSERT INTO offline_jobs (
          id, job_reference, audit_event_id, account_number, customer_name,
          address, gps_lat, gps_lng, anomaly_type, alert_level, status,
          scheduled_at, notes, estimated_variance_ghs, synced_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
        ON CONFLICT(id) DO UPDATE SET
          status = excluded.status,
          notes = excluded.notes,
          synced_at = datetime('now'),
          updated_at = datetime('now')`,
        [
          job.id,
          job.job_reference || job.id,
          job.audit_event_id || null,
          job.account_number,
          job.customer_name,
          job.address,
          job.gps_lat,
          job.gps_lng,
          job.anomaly_type || null,
          job.alert_level,
          job.status,
          job.scheduled_at || null,
          job.notes || null,
          job.estimated_variance_ghs || null,
        ]
      )
    }
  })
}

/**
 * Get all cached jobs from SQLite (used when offline)
 */
export async function getCachedJobs(): Promise<FieldJob[]> {
  const database = await initOfflineDB()

  const rows = await database.getAllAsync<any>(
    `SELECT * FROM offline_jobs
     WHERE status NOT IN ('COMPLETED', 'CANCELLED')
     ORDER BY
       CASE alert_level
         WHEN 'CRITICAL' THEN 1
         WHEN 'HIGH' THEN 2
         WHEN 'MEDIUM' THEN 3
         ELSE 4
       END,
       created_at ASC`
  )

  return rows.map(rowToFieldJob)
}

/**
 * Update a job's status in the local cache
 */
export async function updateCachedJobStatus(jobId: string, status: string): Promise<void> {
  const database = await initOfflineDB()
  await database.runAsync(
    `UPDATE offline_jobs SET status = ?, updated_at = datetime('now') WHERE id = ?`,
    [status, jobId]
  )
}

// ─── Pending Submissions ──────────────────────────────────────────────────────

/**
 * Queue a job submission for background sync when offline
 */
export async function queueSubmission(
  jobId: string,
  submission: JobSubmission,
  photoUris: string[]
): Promise<string> {
  const database = await initOfflineDB()
  const submissionId = `sub_${Date.now()}_${Math.random().toString(36).slice(2)}`

  await database.runAsync(
    `INSERT INTO pending_submissions (id, job_id, submission_json, photo_uris, status)
     VALUES (?, ?, ?, ?, 'PENDING')`,
    [submissionId, jobId, JSON.stringify(submission), JSON.stringify(photoUris)]
  )

  // Update local job status to COMPLETED optimistically
  await updateCachedJobStatus(jobId, 'COMPLETED')

  return submissionId
}

/**
 * Get all pending submissions that need to be synced
 */
export async function getPendingSubmissions(): Promise<PendingSubmission[]> {
  const database = await initOfflineDB()

  return database.getAllAsync<PendingSubmission>(
    `SELECT * FROM pending_submissions
     WHERE status = 'PENDING' AND retry_count < 5
     ORDER BY created_at ASC`
  )
}

export interface PendingSubmission {
  id: string
  job_id: string
  submission_json: string
  photo_uris: string
  status: string
  retry_count: number
  last_error?: string
  created_at: string
}

// ─── Photo Storage ────────────────────────────────────────────────────────────

/**
 * Store a captured photo locally before upload
 */
export async function storePhotoLocally(
  jobId: string,
  localUri: string,
  photoHash: string,
  gpsLat: number,
  gpsLng: number,
  gpsAccuracy: number,
  withinFence: boolean
): Promise<string> {
  const database = await initOfflineDB()
  const photoId = `photo_${Date.now()}_${Math.random().toString(36).slice(2)}`

  await database.runAsync(
    `INSERT INTO offline_photos (
      id, job_id, local_uri, photo_hash, gps_lat, gps_lng,
      gps_accuracy, captured_at, within_fence
    ) VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'), ?)`,
    [photoId, jobId, localUri, photoHash, gpsLat, gpsLng, gpsAccuracy, withinFence ? 1 : 0]
  )

  return photoId
}

/**
 * Get all unuploaded photos for a job
 */
export async function getUnuploadedPhotos(jobId: string): Promise<OfflinePhoto[]> {
  const database = await initOfflineDB()
  return database.getAllAsync<OfflinePhoto>(
    `SELECT * FROM offline_photos WHERE job_id = ? AND uploaded = 0`,
    [jobId]
  )
}

export interface OfflinePhoto {
  id: string
  job_id: string
  local_uri: string
  photo_hash: string
  gps_lat: number
  gps_lng: number
  gps_accuracy: number
  captured_at: string
  within_fence: number
  uploaded: number
  remote_url?: string
}

// ─── Background Sync ──────────────────────────────────────────────────────────

/**
 * SyncManager handles background synchronization of pending submissions.
 * Call startBackgroundSync() once on app startup.
 */
export class SyncManager {
  private syncInterval: ReturnType<typeof setInterval> | null = null
  private isSyncing = false

  /**
   * Start the background sync loop (runs every 30 seconds when online)
   */
  startBackgroundSync(): void {
    // Listen for network state changes
    NetInfo.addEventListener((state) => {
      if (state.isConnected && !this.isSyncing) {
        this.syncPendingSubmissions()
      }
    })

    // Also poll every 30 seconds
    this.syncInterval = setInterval(() => {
      this.syncPendingSubmissions()
    }, 30_000)
  }

  stopBackgroundSync(): void {
    if (this.syncInterval) {
      clearInterval(this.syncInterval)
      this.syncInterval = null
    }
  }

  /**
   * Attempt to upload all pending submissions
   */
  async syncPendingSubmissions(): Promise<SyncResult> {
    if (this.isSyncing) return { synced: 0, failed: 0, skipped: 0 }

    const netState = await NetInfo.fetch()
    if (!netState.isConnected) {
      return { synced: 0, failed: 0, skipped: 0 }
    }

    this.isSyncing = true
    const result: SyncResult = { synced: 0, failed: 0, skipped: 0 }

    try {
      const database = await initOfflineDB()
      const pending = await getPendingSubmissions()

      for (const sub of pending) {
        try {
          const submission: JobSubmission = JSON.parse(sub.submission_json)
          const photoUris: string[] = JSON.parse(sub.photo_uris)

          // Upload photos first
          const uploadedPhotoUrls: string[] = []
          for (const uri of photoUris) {
            try {
              const formData = new FormData()
              formData.append('photo', {
                uri,
                type: 'image/jpeg',
                name: `meter_${Date.now()}.jpg`,
              } as any)
              formData.append('job_id', sub.job_id)

              const uploadResp = await apiClient.post('/api/v1/ocr/upload', formData, {
                headers: { 'Content-Type': 'multipart/form-data' },
                timeout: 60_000,
              })
              uploadedPhotoUrls.push(uploadResp.data.url)
            } catch (photoErr) {
              console.warn('Photo upload failed, using local URI', photoErr)
              uploadedPhotoUrls.push(uri)
            }
          }

          // Submit the job evidence
          const enrichedSubmission = {
            ...submission,
            photo_urls: uploadedPhotoUrls,
          }

          await apiClient.post(`/api/v1/field-jobs/${sub.job_id}/submit`, enrichedSubmission)

          // Mark as synced
          await database.runAsync(
            `UPDATE pending_submissions SET status = 'SYNCED', attempted_at = datetime('now') WHERE id = ?`,
            [sub.id]
          )

          // Mark photos as uploaded
          await database.runAsync(
            `UPDATE offline_photos SET uploaded = 1 WHERE job_id = ?`,
            [sub.job_id]
          )

          result.synced++
        } catch (err: any) {
          const errorMsg = err?.message || 'Unknown error'
          await database.runAsync(
            `UPDATE pending_submissions
             SET retry_count = retry_count + 1,
                 last_error = ?,
                 attempted_at = datetime('now'),
                 status = CASE WHEN retry_count >= 4 THEN 'FAILED' ELSE 'PENDING' END
             WHERE id = ?`,
            [errorMsg, sub.id]
          )
          result.failed++
        }
      }
    } finally {
      this.isSyncing = false
    }

    return result
  }

  /**
   * Get sync statistics for display in the UI
   */
  async getSyncStats(): Promise<SyncStats> {
    const database = await initOfflineDB()

    const pending = await database.getFirstAsync<{ count: number }>(
      `SELECT COUNT(*) as count FROM pending_submissions WHERE status = 'PENDING'`
    )
    const failed = await database.getFirstAsync<{ count: number }>(
      `SELECT COUNT(*) as count FROM pending_submissions WHERE status = 'FAILED'`
    )
    const unuploadedPhotos = await database.getFirstAsync<{ count: number }>(
      `SELECT COUNT(*) as count FROM offline_photos WHERE uploaded = 0`
    )
    const cachedJobs = await database.getFirstAsync<{ count: number }>(
      `SELECT COUNT(*) as count FROM offline_jobs`
    )

    return {
      pendingSubmissions: pending?.count ?? 0,
      failedSubmissions: failed?.count ?? 0,
      unuploadedPhotos: unuploadedPhotos?.count ?? 0,
      cachedJobs: cachedJobs?.count ?? 0,
    }
  }
}

export interface SyncResult {
  synced: number
  failed: number
  skipped: number
}

export interface SyncStats {
  pendingSubmissions: number
  failedSubmissions: number
  unuploadedPhotos: number
  cachedJobs: number
}

// Singleton sync manager
export const syncManager = new SyncManager()

// ─── Helpers ──────────────────────────────────────────────────────────────────

function rowToFieldJob(row: any): FieldJob {
  return {
    id: row.id,
    job_reference: row.job_reference,
    audit_event_id: row.audit_event_id,
    account_number: row.account_number,
    customer_name: row.customer_name,
    address: row.address,
    gps_lat: row.gps_lat,
    gps_lng: row.gps_lng,
    anomaly_type: row.anomaly_type,
    alert_level: row.alert_level,
    status: row.status,
    scheduled_at: row.scheduled_at,
    notes: row.notes,
    estimated_variance_ghs: row.estimated_variance_ghs,
  }
}

/**
 * Check if the device is currently online
 */
export async function isOnline(): Promise<boolean> {
  const state = await NetInfo.fetch()
  return state.isConnected === true
}

/**
 * Clear all offline data (call on logout)
 */
export async function clearOfflineData(): Promise<void> {
  const database = await initOfflineDB()
  await database.execAsync(`
    DELETE FROM pending_submissions;
    DELETE FROM offline_photos;
    DELETE FROM offline_jobs;
  `)
}
