import { useCallback, useRef } from "react"
import { fetchGalleryImages } from "@/api/endpoints"
import type { GalleryImageDTO, GalleryImagesResponse } from "@/types"
import { useInfiniteScroll } from "./useInfiniteScroll"

const PAGE_SIZE = 50

export function useGalleryImages(view: string) {
  const viewRef = useRef(view)

  // Keep viewRef in sync
  if (viewRef.current !== view) {
    viewRef.current = view
  }

  const { items, total, hasMore, isLoading, error, initialized, loadMore, reset } =
    useInfiniteScroll<GalleryImageDTO, GalleryImagesResponse>({
      fetchFn: (page, pageSize) => fetchGalleryImages(page, pageSize, viewRef.current),
      pageSize: PAGE_SIZE,
      transform: (response) => response.images,
      responseTotal: (response) => response.totalImages,
      responseHasNext: (response) => response.hasNextPage,
    })

  const resetWithView = useCallback(
    (newView?: string) => {
      if (newView !== undefined) {
        viewRef.current = newView
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
    reset: resetWithView,
  }
}
