package service

// GeocodingService resolves GPS coordinates from street addresses.
//
// Q6: GWL GIS / Meter Coordinates fallback
//
// When GWL provides customer addresses but not GPS coordinates, this service
// attempts to geocode the address using OpenStreetMap Nominatim (free, no API key).
//
// Strategy (in priority order):
//  1. GWL_PROVIDED:    Use coordinates from GWL GIS export (highest confidence)
//  2. GEOCODED_OSM:    Geocode from address via OpenStreetMap Nominatim
//  3. FIELD_CONFIRMED: Field officer confirms/corrects GPS on first visit
//  4. MANUAL_ADMIN:    System Admin manually enters coordinates
//
// GPS fence behaviour when coordinates are missing:
//  - If gps_source = GEOCODED_OSM: fence radius is expanded to 50m (vs 5m default)
//    because geocoded addresses are less precise than physical meter locations.
//  - If gps_source = FIELD_CONFIRMED: use standard 5m fence.
//  - If no coordinates at all: field officer must use "GPS Capture" mode on first
//    visit, which records their GPS as the meter location (FIELD_CONFIRMED).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// GeocodingService geocodes addresses using OpenStreetMap Nominatim
type GeocodingService struct {
	client  *http.Client
	baseURL string
	db      *pgxpool.Pool
	logger  *zap.Logger
}

// GeocodingResult holds the result of a geocoding attempt
type GeocodingResult struct {
	AccountID    uuid.UUID `json:"account_id"`
	AccountNum   string    `json:"account_number"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	Quality      float64   `json:"quality"`
	Source       string    `json:"source"`
	DisplayName  string    `json:"display_name"`
	FenceRadiusM float64   `json:"fence_radius_m"`
}

type nominatimResult struct {
	Lat         string  `json:"lat"`
	Lon         string  `json:"lon"`
	DisplayName string  `json:"display_name"`
	Importance  float64 `json:"importance"`
}

func NewGeocodingService(db *pgxpool.Pool, logger *zap.Logger) *GeocodingService {
	return &GeocodingService{
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: "https://nominatim.openstreetmap.org",
		db:      db,
		logger:  logger,
	}
}

// GeocodeAccount attempts to resolve GPS coordinates for an account from its address.
func (g *GeocodingService) GeocodeAccount(
	ctx context.Context,
	accountID uuid.UUID,
) (*GeocodingResult, error) {
	var accountNum, addr1, addr2, gpsSource string
	var existingLat, existingLng *float64

	err := g.db.QueryRow(ctx, `
		SELECT wa.gwl_account_number,
		       COALESCE(wa.address_line1, ''),
		       COALESCE(wa.address_line2, ''),
		       wa.gps_latitude,
		       wa.gps_longitude,
		       wa.gps_source::text
		FROM water_accounts wa
		WHERE wa.id = $1
	`, accountID).Scan(&accountNum, &addr1, &addr2, &existingLat, &existingLng, &gpsSource)
	if err != nil {
		// Fallback: gps_source column may not exist on older DB schema (pre-migration-029)
		err2 := g.db.QueryRow(ctx, `
			SELECT wa.gwl_account_number,
			       COALESCE(wa.address_line1, ''),
			       COALESCE(wa.address_line2, ''),
			       wa.gps_latitude,
			       wa.gps_longitude,
			       'UNKNOWN'
			FROM water_accounts wa
			WHERE wa.id = $1
		`, accountID).Scan(&accountNum, &addr1, &addr2, &existingLat, &existingLng, &gpsSource)
		if err2 != nil {
			return nil, fmt.Errorf("account not found: %w", err2)
		}
	}

	if gpsSource == "GWL_PROVIDED" || gpsSource == "FIELD_CONFIRMED" {
		return nil, fmt.Errorf("account already has %s coordinates — not overwriting", gpsSource)
	}

	searchQuery := buildGhanaSearchQuery(addr1, addr2)
	if searchQuery == "" {
		return nil, fmt.Errorf("account %s has no address to geocode", accountNum)
	}

	lat, lng, quality, displayName, err := g.callNominatim(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("Nominatim geocoding failed: %w", err)
	}

	fenceRadius := 50.0
	if quality >= 80 {
		fenceRadius = 25.0
	}

	_, err = g.db.Exec(ctx, `
		UPDATE water_accounts SET
			gps_latitude        = $1,
			gps_longitude       = $2,
			gps_source          = 'GEOCODED_OSM'::gps_source_type,
			gps_geocode_quality = $3,
			gps_geocoded_at     = NOW(),
			gps_fence_radius_m  = $4,
			updated_at          = NOW()
		WHERE id = $5
	`, lat, lng, quality, fenceRadius, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to update account GPS: %w", err)
	}

	g.logger.Info("Account geocoded",
		zap.String("account_num", accountNum),
		zap.Float64("lat", lat),
		zap.Float64("lng", lng),
		zap.Float64("quality", quality),
	)

	return &GeocodingResult{
		AccountID:    accountID,
		AccountNum:   accountNum,
		Latitude:     lat,
		Longitude:    lng,
		Quality:      quality,
		Source:       "GEOCODED_OSM",
		DisplayName:  displayName,
		FenceRadiusM: fenceRadius,
	}, nil
}

// ConfirmGPSFromField upgrades an account's GPS to FIELD_CONFIRMED with 5m fence.
func (g *GeocodingService) ConfirmGPSFromField(
	ctx context.Context,
	accountID uuid.UUID,
	lat, lng float64,
	confirmedByUserID uuid.UUID,
) error {
	_, err := g.db.Exec(ctx, `
		UPDATE water_accounts SET
			gps_latitude        = $1,
			gps_longitude       = $2,
			gps_source          = 'FIELD_CONFIRMED'::gps_source_type,
			gps_geocode_quality = 99.0,
			gps_fence_radius_m  = 5.0,
			gps_confirmed_at    = NOW(),
			gps_confirmed_by    = $3,
			updated_at          = NOW()
		WHERE id = $4
	`, lat, lng, confirmedByUserID, accountID)
	return err
}

// GeocodeDistrict geocodes all address-only accounts in a district (rate-limited).
func (g *GeocodingService) GeocodeDistrict(
	ctx context.Context,
	districtID uuid.UUID,
) (int, int, error) {
	rows, err := g.db.Query(ctx, `
		SELECT id FROM water_accounts
		WHERE district_id = $1
		  AND gps_latitude IS NULL
		  AND address_line1 IS NOT NULL AND address_line1 != ''
		  AND gps_source NOT IN ('GWL_PROVIDED', 'FIELD_CONFIRMED')
		ORDER BY gwl_account_number
		LIMIT 500
	`, districtID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}

	success, failed := 0, 0
	for _, id := range ids {
		select {
		case <-ctx.Done():
			return success, failed, ctx.Err()
		default:
		}
		if _, err := g.GeocodeAccount(ctx, id); err != nil {
			failed++
		} else {
			success++
		}
		time.Sleep(1100 * time.Millisecond) // Nominatim: 1 req/sec
	}
	return success, failed, nil
}

func (g *GeocodingService) callNominatim(
	ctx context.Context,
	query string,
) (float64, float64, float64, string, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("limit", "1")
	params.Set("countrycodes", "gh")

	reqURL := fmt.Sprintf("%s/search?%s", g.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, 0, 0, "", err
	}
	req.Header.Set("User-Agent", "GN-WAAS/1.0 (Ghana National Water Audit; contact@gnwaas.gov.gh)")

	resp, err := g.client.Do(req)
	if err != nil {
		return 0, 0, 0, "", err
	}
	defer resp.Body.Close()

	var results []nominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return 0, 0, 0, "", err
	}
	if len(results) == 0 {
		return 0, 0, 0, "", fmt.Errorf("no results for: %s", query)
	}

	r := results[0]
	var lat, lng float64
	fmt.Sscanf(r.Lat, "%f", &lat)
	fmt.Sscanf(r.Lon, "%f", &lng)

	quality := r.Importance * 100
	if quality > 100 {
		quality = 100
	}
	if quality < 10 {
		quality = 10
	}
	return lat, lng, quality, r.DisplayName, nil
}

func buildGhanaSearchQuery(addr1, addr2 string) string {
	q := ""
	if addr1 != "" {
		q = addr1
	}
	if addr2 != "" {
		if q != "" {
			q += ", "
		}
		q += addr2
	}
	if q == "" {
		return ""
	}
	return q + ", Ghana"
}
