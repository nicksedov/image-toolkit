import { useCallback, useEffect, useRef } from "react"
import { fetchGalleryImages } from "@/api/endpoints"
import type { GalleryImageDTO, GalleryImagesResponse } from "@/types"
import { useInfiniteScroll } from "./useInfiniteScroll"

const PAGE_SIZE = 50

export function useGalleryImages(view: string, sortOrder: string = "newest") {
  const viewRef = useRef(view)
  const sortOrderRef = useRef(sortOrder)

  const { items, total, hasMore, isLoading, error, initialized, loadMore, reset, removeItem } =
    useInfiniteScroll<GalleryImageDTO, GalleryImagesResponse>({
      fetchFn: (page, pageSize) => fetchGalleryImages(page, pageSize, viewRef.current, sortOrderRef.current),
      pageSize: PAGE_SIZE,
      transform: (response) => response.images,
      responseTotal: (response) => response.totalImages,
      responseHasNext: (response) => response.hasNextPage,
    })

  // Reset and reload when view or sortOrder changes
  useEffect(() => {
    if (viewRef.current !== view || sortOrderRef.current !== sortOrder) {
      viewRef.current = view
      sortOrderRef.current = sortOrder
      reset()
    }
  }, [view, sortOrder, reset])

  const resetWithView = useCallback(
    (newView?: string, newSortOrder?: string) => {
      if (newView !== undefined) {
        viewRef.current = newView
      }
      if (newSortOrder !== undefined) {
        sortOrderRef.current = newSortOrder
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
    removeImage: removeItem,
  }
}
