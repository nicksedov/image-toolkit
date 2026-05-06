import { useCallback, useRef } from "react"
import { fetchGeoImages } from "@/api/endpoints"
import type { GalleryImageDTO, GeoImagesResponse } from "@/types"
import { useInfiniteScroll } from "./useInfiniteScroll"

const PAGE_SIZE = 50

interface GeoBounds {
  minLat: number
  maxLat: number
  minLng: number
  maxLng: number
}

export function useGeoImages(bounds: GeoBounds | null) {
  const boundsRef = useRef(bounds)

  // Keep boundsRef in sync
  if (boundsRef.current !== bounds) {
    boundsRef.current = bounds
  }

  const { items, total, hasMore, isLoading, error, initialized, loadMore, reset } =
    useInfiniteScroll<GalleryImageDTO, GeoImagesResponse>({
      fetchFn: (page, pageSize) => {
        if (!boundsRef.current) {
          return Promise.resolve({
            images: [],
            totalImages: 0,
            currentPage: page,
            pageSize,
            totalPages: 0,
            hasNextPage: false,
          })
        }
        return fetchGeoImages(page, pageSize, boundsRef.current)
      },
      pageSize: PAGE_SIZE,
      transform: (response) => response.images,
      responseTotal: (response) => response.totalImages,
      responseHasNext: (response) => response.hasNextPage,
    })

  const resetWithBounds = useCallback(
    (newBounds?: GeoBounds | null) => {
      if (newBounds !== undefined) {
        boundsRef.current = newBounds
      }
      reset()
    },
    [reset]
  )

  return {
    images: items,
    totalImages: total,
    hasMore,
    isLoading,
    error,
    initialized,
    loadMore,
    reset: resetWithBounds,
  }
}
