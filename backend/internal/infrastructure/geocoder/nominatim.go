package geocoder

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// NominatimResult represents a single result from the Nominatim geocoding API.
type NominatimResult struct {
	Lat         float64
	Lon         float64
	DisplayName string
	Type        string
}

// NominatimClient provides forward geocoding via the Nominatim (OpenStreetMap) REST API.
type NominatimClient struct {
	httpClient *http.Client
	baseURL    string
}

// nominatimJSONResult is the raw JSON structure returned by Nominatim.
type nominatimJSONResult struct {
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
}

// NewNominatimClient creates a NominatimClient with sensible defaults.
// An optional custom httpClient can be provided for testing (e.g. with httptest.Server).
func NewNominatimClient(httpClient *http.Client, baseURL string) *NominatimClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if baseURL == "" {
		baseURL = "https://nominatim.openstreetmap.org"
	}
	return &NominatimClient{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

// ReverseGeocodeResult holds the bilingual location names from a reverse geocode call.
type ReverseGeocodeResult struct {
	NameLocal string // Local-language place name (city/town/state from address details)
	NameEng   string // English display name (from display_name field)
}

// nominatimReverseJSON is the raw JSON structure returned by Nominatim /reverse.
type nominatimReverseJSON struct {
	DisplayName string `json:"display_name"`
	Address     struct {
		City     string `json:"city"`
		Town     string `json:"town"`
		Village  string `json:"village"`
		State    string `json:"state"`
		Country  string `json:"country"`
		Hamlet   string `json:"hamlet"`
		Municipality string `json:"municipality"`
	} `json:"address"`
}

// ReverseGeocode performs reverse geocoding for the given coordinates.
// Returns a ReverseGeocodeResult with local and English location names.
func (n *NominatimClient) ReverseGeocode(lat, lng float64) (*ReverseGeocodeResult, error) {
	reverseURL := fmt.Sprintf("%s/reverse?lat=%f&lon=%f&format=json&zoom=10&addressdetails=1&accept-language=en",
		n.baseURL, lat, lng)

	req, err := http.NewRequest(http.MethodGet, reverseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create reverse request: %w", err)
	}
	req.Header.Set("User-Agent", "ImageToolkit/1.0 (https://github.com/nicksedov/image-toolkit)")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nominatim reverse request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim reverse returned status %d", resp.StatusCode)
	}

	var raw nominatimReverseJSON
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode nominatim reverse response: %w", err)
	}

	// Build local name from address details (city > town > village > hamlet > municipality > state)
	nameLocal := raw.Address.City
	if nameLocal == "" {
		nameLocal = raw.Address.Town
	}
	if nameLocal == "" {
		nameLocal = raw.Address.Village
	}
	if nameLocal == "" {
		nameLocal = raw.Address.Hamlet
	}
	if nameLocal == "" {
		nameLocal = raw.Address.Municipality
	}
	if nameLocal == "" {
		nameLocal = raw.Address.State
	}

	return &ReverseGeocodeResult{
		NameLocal: nameLocal,
		NameEng:   raw.DisplayName,
	}, nil
}

// Search performs a forward geocoding search and returns up to 10 results.
func (n *NominatimClient) Search(query string) ([]NominatimResult, error) {
	searchURL := fmt.Sprintf("%s/search?q=%s&format=json&limit=10",
		n.baseURL, url.QueryEscape(query))

	req, err := http.NewRequest(http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "ImageToolkit/1.0 (https://github.com/nicksedov/image-toolkit)")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nominatim request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim returned status %d", resp.StatusCode)
	}

	var rawResults []nominatimJSONResult
	if err := json.NewDecoder(resp.Body).Decode(&rawResults); err != nil {
		return nil, fmt.Errorf("failed to decode nominatim response: %w", err)
	}

	results := make([]NominatimResult, 0, len(rawResults))
	for _, raw := range rawResults {
		lat, err := strconv.ParseFloat(raw.Lat, 64)
		if err != nil {
			continue
		}
		lon, err := strconv.ParseFloat(raw.Lon, 64)
		if err != nil {
			continue
		}
		results = append(results, NominatimResult{
			Lat:         lat,
			Lon:         lon,
			DisplayName: raw.DisplayName,
			Type:        raw.Type,
		})
	}

	return results, nil
}
