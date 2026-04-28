import { useCallback, useRef, useState } from "react"
import { fetchOcrDocuments } from "@/api/endpoints"
import type { OcrDocumentDTO, OcrDocumentsResponse } from "@/types"

const PAGE_SIZE = 50

export function useOcrDocuments() {
  const [documents, setDocuments] = useState<OcrDocumentDTO[]>([])
  const [totalDocuments, setTotalDocuments] = useState(0)
  const [hasMore, setHasMore] = useState(true)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const pageRef = useRef(1)

  // Track if we've done at least one load
  const [initialized, setInitialized] = useState(false)

  // Prefetch buffer for the next page
  const prefetchRef = useRef<{
    page: number
    promise: Promise<OcrDocumentsResponse> | null
    data: OcrDocumentsResponse | null
  }>({ page: 0, promise: null, data: null })

  const startPrefetch = useCallback((page: number) => {
    const buf = prefetchRef.current
    if (buf.page === page && (buf.data || buf.promise)) {
      return // already prefetching/prefetched this page
    }
    buf.page = page
    buf.data = null
    buf.promise = fetchOcrDocuments(page, PAGE_SIZE)
      .then((result) => {
        // Only store if still relevant (page hasn't changed)
        if (prefetchRef.current.page === page) {
          prefetchRef.current.data = result
        }
        return result
      })
      .catch(() => {
        // Silently discard prefetch errors; the real load will handle them
        prefetchRef.current.promise = null
        return null as unknown as OcrDocumentsResponse
      })
  }, [])

  const consumePrefetch = useCallback((page: number): OcrDocumentsResponse | null => {
    const buf = prefetchRef.current
    if (buf.page === page && buf.data) {
      const data = buf.data
      buf.page = 0
      buf.data = null
      buf.promise = null
      return data
    }
    return null
  }, [])

  const loadMore = useCallback(async () => {
    if (isLoading) return
    setIsLoading(true)
    setError(null)
    try {
      const currentPage = pageRef.current

      // Use prefetched data if available
      const prefetched = consumePrefetch(currentPage)
      const result = prefetched ?? await fetchOcrDocuments(currentPage, PAGE_SIZE)

      setDocuments((prev) => {
        // Avoid duplicates by checking IDs
        const existingIds = new Set(prev.map((doc) => doc.id))
        const newDocs = result.documents.filter((doc) => !existingIds.has(doc.id))
        return [...prev, ...newDocs]
      })
      setTotalDocuments(result.total)
      setHasMore(result.hasNextPage)
      pageRef.current += 1
      setInitialized(true)

      // Prefetch the next page in background
      if (result.hasNextPage) {
        startPrefetch(pageRef.current)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load documents")
    } finally {
      setIsLoading(false)
    }
  }, [isLoading, consumePrefetch, startPrefetch])

  const reset = useCallback(() => {
    setDocuments([])
    setTotalDocuments(0)
    setHasMore(true)
    setError(null)
    pageRef.current = 1
    setInitialized(false)
    // Invalidate prefetch buffer
    prefetchRef.current = { page: 0, promise: null, data: null }
  }, [])

  return {
    documents,
    totalDocuments,
    hasMore,
    isLoading,
    error,
    initialized,
    loadMore,
    reset,
  }
}
