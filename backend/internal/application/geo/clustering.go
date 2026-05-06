package geo

import (
	"fmt"
	"math"

	"image-toolkit/internal/interfaces/dto"

	goclusterlib "github.com/MadAppGang/gocluster"
	"gorm.io/gorm"
)

// ClusterParams holds the viewport parameters for clustering
type ClusterParams struct {
	MinLat, MaxLat, MinLng, MaxLng float64
	Zoom                           int
	ViewportWidth, ViewportHeight  int
}

// imageGeoPoint implements goclusterlib.GeoPoint interface
type imageGeoPoint struct {
	lat, lng float64
	path     string
	index    int
}

func (p *imageGeoPoint) GetCoordinates() goclusterlib.GeoCoordinates {
	return goclusterlib.GeoCoordinates{Lon: p.lng, Lat: p.lat}
}

// ComputeClusters performs server-side clustering using goclusterlib library.
// It uses hierarchical grid-based clustering with KD-tree indexing for fast queries.
func ComputeClusters(db *gorm.DB, params ClusterParams) ([]dto.GeoCluster, int, error) {
	// Query all images with GPS coordinates within the viewport bounds
	type imageWithGPS struct {
		Path         string
		GPSLatitude  float64
		GPSLongitude float64
	}

	var images []imageWithGPS
	err := db.Table("image_files").
		Select("image_files.path, image_metadata.gps_latitude, image_metadata.gps_longitude").
		Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
		Where("image_metadata.gps_latitude IS NOT NULL").
		Where("image_metadata.gps_longitude IS NOT NULL").
		Where("image_metadata.gps_latitude BETWEEN ? AND ?", params.MinLat, params.MaxLat).
		Where("image_metadata.gps_longitude BETWEEN ? AND ?", params.MinLng, params.MaxLng).
		Find(&images).Error

	if err != nil {
		return nil, 0, err
	}

	totalImages := len(images)
	if totalImages == 0 {
		return []dto.GeoCluster{}, 0, nil
	}

	// Convert to goclusterlib.GeoPoint slice
	points := make([]goclusterlib.GeoPoint, len(images))
	for i, img := range images {
		points[i] = &imageGeoPoint{
			lat:   img.GPSLatitude,
			lng:   img.GPSLongitude,
			path:  img.Path,
			index: i,
		}
	}

	// Create cluster with configuration
	cluster := goclusterlib.NewCluster()

	// Configure clustering parameters
	// PointSize: cluster radius in pixels (controls how close points merge)
	// MinZoom/MaxZoom: zoom range for clustering hierarchy
	cluster.PointSize = 40
	cluster.MinZoom = 0
	cluster.MaxZoom = 18

	// Build the clustering index
	if err := cluster.ClusterPoints(points); err != nil {
		return nil, 0, err
	}

	// Query clusters for the current viewport
	northWest := goclusterlib.GeoPoint(&imageGeoPoint{
		lat: params.MaxLat,
		lng: params.MinLng,
	})
	southEast := goclusterlib.GeoPoint(&imageGeoPoint{
		lat: params.MinLat,
		lng: params.MaxLng,
	})

	clusterPoints := cluster.GetClusters(northWest, southEast, params.Zoom)

	// Convert cluster points to DTO
	// goclusterlib returns:
	// - Individual points with Id = original index, NumPoints = 1
	// - Clustered groups with Id >= ClusterIdxSeed, NumPoints > 1
	clusters := make([]dto.GeoCluster, 0, len(clusterPoints))

	for _, cp := range clusterPoints {
		// Get geographic coordinates from mercator projection
		lat, lng := cp.Coordinates()

		// Skip individual points (NumPoints == 1) unless zoomed in very far
		// This reduces clutter at lower zoom levels
		if cp.NumPoints == 1 && params.Zoom < 15 {
			continue
		}

		clusterID := ""
		if cp.NumPoints > 1 {
			// This is a cluster group
			clusterID = fmt.Sprintf("cluster_%d", cp.Id)
		} else {
			// Single image point
			clusterID = fmt.Sprintf("point_%d", cp.Id)
		}

		// For clusters, we need to gather image paths
		// goclusterlib doesn't store original data in ClusterPoint, only coordinates
		// So we'll compute a bounding area and fetch nearby images
		var imagePaths []string
		if cp.NumPoints > 1 {
			// For clusters, we'll need to query the database for images near this point
			// Use a small radius based on the cluster size
			radius := calculateClusterRadius(cp.NumPoints, params.Zoom)

			var paths []string
			db.Table("image_files").
				Select("image_files.path").
				Joins("INNER JOIN image_metadata ON image_metadata.image_file_id = image_files.id").
				Where("image_metadata.gps_latitude BETWEEN ? AND ?", lat-radius, lat+radius).
				Where("image_metadata.gps_longitude BETWEEN ? AND ?", lng-radius, lng+radius).
				Limit(500).
				Pluck("path", &paths)

			imagePaths = paths
		}

		clusters = append(clusters, dto.GeoCluster{
			ID:         clusterID,
			Latitude:   lat,
			Longitude:  lng,
			Count:      cp.NumPoints,
			ImagePaths: imagePaths,
		})
	}

	return clusters, totalImages, nil
}

// calculateClusterRadius estimates the geographic radius for a cluster based on zoom level
func calculateClusterRadius(numPoints int, zoom int) float64 {
	// Base radius decreases with zoom level
	// At zoom 0: ~10 degrees, at zoom 18: ~0.001 degrees
	baseRadius := 10.0 / math.Pow(2, float64(zoom))

	// Larger clusters get a slightly larger radius
	if numPoints > 10 {
		baseRadius *= 1.5
	}

	return baseRadius
}
