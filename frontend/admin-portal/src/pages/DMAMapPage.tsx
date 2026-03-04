import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import api from '../lib/api-client'

// Leaflet is loaded dynamically to avoid SSR issues with Vite
// The CSS must be imported for map tiles to render correctly
import 'leaflet/dist/leaflet.css'

interface District {
  id: string
  district_code: string
  district_name: string
  region: string
  zone_type: string
  is_pilot_district: boolean
  is_active: boolean
  gps_latitude?: number
  gps_longitude?: number
  loss_ratio_pct?: number
  total_connections?: number
  open_anomalies?: number
}

interface DMABoundary {
  district_id: string
  geojson: GeoJSON.FeatureCollection
}

// Ghana's geographic center — default map view
const GHANA_CENTER: [number, number] = [7.9465, -1.0232]
const GHANA_ZOOM = 7

// Color scale for NRW percentage (green → yellow → red)
function nrwColor(pct: number): string {
  if (pct < 20) return '#22c55e'   // green  — good
  if (pct < 35) return '#f59e0b'   // amber  — warning
  if (pct < 50) return '#f97316'   // orange — high
  return '#ef4444'                  // red    — critical
}

export default function DMAMapPage() {
  const mapRef = useRef<HTMLDivElement>(null)
  const leafletMap = useRef<import('leaflet').Map | null>(null)
  const markersRef = useRef<import('leaflet').LayerGroup | null>(null)
  const [selectedDistrict, setSelectedDistrict] = useState<District | null>(null)
  const [mapReady, setMapReady] = useState(false)

  const { data: districts = [], isLoading } = useQuery<District[]>({
    queryKey: ['districts-map'],
    queryFn: async () => {
      const res = await api.get('/districts')
      return res.data?.data ?? []
    },
  })

  // Initialise Leaflet map once the container div is mounted
  useEffect(() => {
    if (!mapRef.current || leafletMap.current) return

    import('leaflet').then((L) => {
      // Fix default marker icon paths broken by Vite bundling
      delete (L.Icon.Default.prototype as unknown as Record<string, unknown>)._getIconUrl
      L.Icon.Default.mergeOptions({
        iconRetinaUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png',
        iconUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png',
        shadowUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png',
      })

      const map = L.map(mapRef.current!, {
        center: GHANA_CENTER,
        zoom: GHANA_ZOOM,
        zoomControl: true,
      })

      // OpenStreetMap tile layer (free, no API key required)
      L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
        maxZoom: 18,
      }).addTo(map)

      markersRef.current = L.layerGroup().addTo(map)
      leafletMap.current = map
      setMapReady(true)
    })

    return () => {
      leafletMap.current?.remove()
      leafletMap.current = null
    }
  }, [])

  // Add district markers whenever districts data or map changes
  useEffect(() => {
    if (!mapReady || !leafletMap.current || !markersRef.current) return

    import('leaflet').then((L) => {
      markersRef.current!.clearLayers()

      districts.forEach((district, idx) => {
        // Use approximate Ghana district coordinates if not provided
        // In production these come from the districts table lat/lng columns
        const lat = district.gps_latitude ?? GHANA_CENTER[0] + (idx % 5) * 0.8 - 2
        const lng = district.gps_longitude ?? GHANA_CENTER[1] + Math.floor(idx / 5) * 0.8 - 2
        const nrw = district.loss_ratio_pct ?? 0
        const color = nrwColor(nrw)

        const marker = L.circleMarker([lat, lng], {
          radius: 12,
          fillColor: color,
          color: '#fff',
          weight: 2,
          opacity: 1,
          fillOpacity: 0.85,
        })

        marker.bindTooltip(
          `<strong>${district.district_name}</strong><br/>` +
          `NRW: ${nrw.toFixed(1)}%<br/>` +
          `Connections: ${district.total_connections?.toLocaleString() ?? 'N/A'}<br/>` +
          `Open Anomalies: ${district.open_anomalies ?? 0}`,
          { permanent: false, direction: 'top' }
        )

        marker.on('click', () => setSelectedDistrict(district))
        markersRef.current!.addLayer(marker)
      })
    })
  }, [mapReady, districts])

  return (
    <div className="flex flex-col h-full gap-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">DMA Map</h1>
          <p className="text-sm text-gray-500 mt-1">
            District Metered Area visualization — NRW heat map across Ghana
          </p>
        </div>
        {/* Legend */}
        <div className="flex items-center gap-4 text-xs text-gray-600">
          <span className="font-medium">NRW Level:</span>
          {[
            { color: '#22c55e', label: '< 20% Good' },
            { color: '#f59e0b', label: '20–35% Warning' },
            { color: '#f97316', label: '35–50% High' },
            { color: '#ef4444', label: '> 50% Critical' },
          ].map(({ color, label }) => (
            <span key={label} className="flex items-center gap-1">
              <span className="w-3 h-3 rounded-full inline-block" style={{ backgroundColor: color }} />
              {label}
            </span>
          ))}
        </div>
      </div>

      <div className="flex gap-4 flex-1 min-h-0">
        {/* Map container */}
        <div className="flex-1 rounded-xl overflow-hidden border border-gray-200 shadow-sm relative">
          {isLoading && (
            <div className="absolute inset-0 bg-white/70 flex items-center justify-center z-10">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
            </div>
          )}
          <div ref={mapRef} className="w-full h-full min-h-[500px]" />
        </div>

        {/* District detail panel */}
        <div className="w-72 flex flex-col gap-3">
          {selectedDistrict ? (
            <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-4">
              <div className="flex items-start justify-between mb-3">
                <div>
                  <h3 className="font-semibold text-gray-900">{selectedDistrict.district_name}</h3>
                  <p className="text-xs text-gray-500">{selectedDistrict.region} Region</p>
                </div>
                <button
                  onClick={() => setSelectedDistrict(null)}
                  className="text-gray-400 hover:text-gray-600 text-lg leading-none"
                >×</button>
              </div>

              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-gray-500">Code</span>
                  <span className="font-mono font-medium">{selectedDistrict.district_code}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">Zone Type</span>
                  <span className="font-medium">{selectedDistrict.zone_type}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">Connections</span>
                  <span className="font-medium">{selectedDistrict.total_connections?.toLocaleString() ?? 'N/A'}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">Open Anomalies</span>
                  <span className={`font-medium ${(selectedDistrict.open_anomalies ?? 0) > 0 ? 'text-red-600' : 'text-green-600'}`}>
                    {selectedDistrict.open_anomalies ?? 0}
                  </span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-gray-500">NRW %</span>
                  <span
                    className="font-bold text-base"
                    style={{ color: nrwColor(selectedDistrict.nrw_percentage ?? 0) }}
                  >
                    {(selectedDistrict.nrw_percentage ?? 0).toFixed(1)}%
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">Status</span>
                  <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${
                    selectedDistrict.is_active
                      ? 'bg-green-100 text-green-700'
                      : 'bg-gray-100 text-gray-600'
                  }`}>
                    {selectedDistrict.is_active ? 'Active' : 'Inactive'}
                  </span>
                </div>
                {selectedDistrict.is_pilot_district && (
                  <div className="mt-2 px-2 py-1 bg-blue-50 text-blue-700 text-xs rounded-md text-center font-medium">
                    ★ Pilot District
                  </div>
                )}
              </div>
            </div>
          ) : (
            <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-4 text-center text-sm text-gray-500">
              <div className="text-3xl mb-2">🗺️</div>
              Click a district marker to view details
            </div>
          )}

          {/* Summary stats */}
          <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-4">
            <h4 className="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">
              National Summary
            </h4>
            <div className="space-y-2 text-sm">
              <div className="flex justify-between">
                <span className="text-gray-500">Total Districts</span>
                <span className="font-medium">{districts.length}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-500">Active</span>
                <span className="font-medium text-green-600">
                  {districts.filter(d => d.is_active).length}
                </span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-500">Pilot Districts</span>
                <span className="font-medium text-blue-600">
                  {districts.filter(d => d.is_pilot_district).length}
                </span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-500">Critical NRW (&gt;50%)</span>
                <span className="font-medium text-red-600">
                  {districts.filter(d => (d.nrw_percentage ?? 0) > 50).length}
                </span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-500">Avg NRW</span>
                <span className="font-medium">
                  {districts.length > 0
                    ? (districts.reduce((s, d) => s + (d.nrw_percentage ?? 0), 0) / districts.length).toFixed(1)
                    : '0.0'}%
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
