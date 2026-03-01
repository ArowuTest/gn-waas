import * as Location from 'expo-location'

export interface GPSReading {
  lat: number
  lng: number
  accuracy: number
  timestamp: string
}

/**
 * Get current GPS position with high accuracy.
 * Requests permission if not already granted.
 */
export async function getCurrentPosition(): Promise<GPSReading> {
  const { status } = await Location.requestForegroundPermissionsAsync()
  if (status !== 'granted') {
    throw new Error('Location permission denied. GPS is required for audit evidence.')
  }

  const location = await Location.getCurrentPositionAsync({
    accuracy: Location.Accuracy.High,
  })

  return {
    lat: location.coords.latitude,
    lng: location.coords.longitude,
    accuracy: location.coords.accuracy ?? 999,
    timestamp: new Date(location.timestamp).toISOString(),
  }
}

/**
 * Haversine distance between two GPS points in metres.
 */
export function haversineDistance(
  lat1: number, lng1: number,
  lat2: number, lng2: number
): number {
  const R = 6371000 // Earth radius in metres
  const dLat = ((lat2 - lat1) * Math.PI) / 180
  const dLng = ((lng2 - lng1) * Math.PI) / 180
  const a =
    Math.sin(dLat / 2) ** 2 +
    Math.cos((lat1 * Math.PI) / 180) *
    Math.cos((lat2 * Math.PI) / 180) *
    Math.sin(dLng / 2) ** 2
  return R * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a))
}

/**
 * Check if officer is within FENCE_RADIUS metres of the meter location.
 */
export function isWithinFence(
  officerLat: number, officerLng: number,
  meterLat: number, meterLng: number,
  fenceRadiusM = 50
): boolean {
  return haversineDistance(officerLat, officerLng, meterLat, meterLng) <= fenceRadiusM
}
