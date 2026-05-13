import { useCallback, useRef, useState } from "react"

export interface UseCursorInfiniteScrollOptions<T, R> {
  /** Fetch function that takes optional cursor and returns response */
  fetchFn: (cursor?: string) => Promise<R>
  /** Transform API response to array of items */
  transform: (response: R) => T[]
  /** Extract next cursor from response */
  responseNextCursor: (response: R) => string | null
  /** Extract total count from response */
  responseTotal?: (response: R) => number
}

export interface UseCursorInfiniteScrollResult<T> {
  /** Accumulated items */
  items: T[]
  /** Total count from last response */
  total: number
  /** Whether there are more pages */
  hasMore: boolean
  /** Whether currently loading */
  isLoading: boolean
  /** Error message if any */
  error: string | null
  /** Whether initial load completed */
  initialized: boolean
  /** Current cursor for next fetch */
  nextCursor: string | null
  /** Load next page */
  loadMore: () => Promise<void>
  /** Reset state */
  reset: () => void
  /** Remove a specific item from the items list */
  removeItem: (key: string | number) => void
}

/**
 * Simplified infinite scroll hook using cursor-based pagination.
 * No prefetch buffer, no duplicate detection - server guarantees uniqueness via cursor.
 */
export function useCursorInfiniteScroll<T, R>(
  options: UseCursorInfiniteScrollOptions<T, R>
): UseCursorInfiniteScrollResult<T> {
  const {
    fetchFn,
    transform,
    responseNextCursor,
    responseTotal,
  } = options

  const [items, setItems] = useState<T[]>([])
  const [total, setTotal] = useState(0)
  const [hasMore, setHasMore] = useState(true)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [initialized, setInitialized] = useState(false)
  const nextCursorRef = useRef<string | null>(null)

  const loadMore = useCallback(async () => {
    if (isLoading) return
    setIsLoading(true)
    setError(null)
    try {
      const cursorArg = nextCursorRef.current ?? undefined
      const result = await fetchFn(cursorArg)

      setItems((prev) => [...prev, ...transform(result)])
      
      const cursor = responseNextCursor(result)
      nextCursorRef.current = cursor
      setHasMore(cursor !== null)

      if (responseTotal) {
        setTotal(responseTotal(result))
      }

      setInitialized(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load data")
    } finally {
      setIsLoading(false)
    }
  }, [isLoading, fetchFn, transform, responseNextCursor, responseTotal])

  const reset = useCallback(() => {
    setItems([])
    setTotal(0)
    setHasMore(true)
    setError(null)
    setInitialized(false)
    nextCursorRef.current = null
  }, [])

  const removeItem = useCallback(
    (key: string | number) => {
      setItems((prev) => prev.filter((item) => (item as any).id !== key))
      setTotal((prev) => Math.max(0, prev - 1))
    },
    []
  )

  return {
    items,
    total,
    hasMore,
    isLoading,
    error,
    initialized,
    nextCursor: nextCursorRef.current,
    loadMore,
    reset,
    removeItem,
  }
}
