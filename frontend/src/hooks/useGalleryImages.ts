import { useCallback, useRef } from "react"
import { fetchGalleryImages } from "@/api/endpoints"
import type { GalleryImageDTO, GalleryImagesResponse } from "@/types"
import { useInfiniteScroll } from "./useInfiniteScroll"

const PAGE_SIZE = 50

export function useGalleryImages(view: string, sortOrder: string = "newest") {
  const viewRef = useRef(view)
  const sortOrderRef = useRef(sortOrder)

  // Keep refs in sync
  if (viewRef.current !== view) {
    viewRef.current = view
  }
  if (sortOrderRef.current !== sortOrder) {
    sortOrderRef.current = sortOrder
  }

  const { items, total, hasMore, isLoading, error, initialized, loadMore, reset, removeItem } =
    useInfiniteScroll<GalleryImageDTO, GalleryImagesResponse>({
      fetchFn: (page, pageSize) => fetchGalleryImages(page, pageSize, viewRef.current, sortOrderRef.current),
      pageSize: PAGE_SIZE,
      transform: (response) => response.images,
      responseTotal: (response) => response.totalImages,
      responseHasNext: (response) => response.hasNextPage,
    })

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
