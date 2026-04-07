import { useCallback, useRef, useState } from "react"
import { fetchGalleryImages } from "@/api/endpoints"
import type { GalleryImageDTO } from "@/types"

const PAGE_SIZE = 50

export function useGalleryImages(view: string) {
  const [images, setImages] = useState<GalleryImageDTO[]>([])
  const [totalImages, setTotalImages] = useState(0)
  const [hasMore, setHasMore] = useState(true)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const pageRef = useRef(1)
  const viewRef = useRef(view)

  // Track if we've done at least one load
  const [initialized, setInitialized] = useState(false)

  const loadMore = useCallback(async () => {
    if (isLoading) return
    setIsLoading(true)
    setError(null)
    try {
      const result = await fetchGalleryImages(pageRef.current, PAGE_SIZE, viewRef.current)
      setImages((prev) => {
        // Avoid duplicates by checking IDs
        const existingIds = new Set(prev.map((img) => img.id))
        const newImages = result.images.filter((img) => !existingIds.has(img.id))
        return [...prev, ...newImages]
      })
      setTotalImages(result.totalImages)
      setHasMore(result.hasNextPage)
      pageRef.current += 1
      setInitialized(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load images")
    } finally {
      setIsLoading(false)
    }
  }, [isLoading])

  const reset = useCallback(
    (newView?: string) => {
      if (newView !== undefined) {
        viewRef.current = newView
      }
      setImages([])
      setTotalImages(0)
      setHasMore(true)
      setError(null)
      pageRef.current = 1
      setInitialized(false)
    },
    []
  )

  return {
    images,
    totalImages,
    hasMore,
    isLoading,
    error,
    initialized,
    loadMore,
    reset,
  }
}
