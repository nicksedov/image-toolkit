import { useState, useEffect, useRef, useCallback } from "react"
import { searchGeocodeLocations } from "@/api/endpoints"
import type { GeocodeSearchResult } from "@/types"

export function useGeocodeSearch() {
  const [query, setQuery] = useState("")
  const [results, setResults] = useState<GeocodeSearchResult[]>([])
  const [isSearching, setIsSearching] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
    }

    if (query.length < 2) {
      return
    }

    timerRef.current = setTimeout(async () => {
      setIsSearching(true)
      try {
        const response = await searchGeocodeLocations(query)
        setResults(response.results)
      } catch {
        setResults([])
      } finally {
        setIsSearching(false)
      }
    }, 300)

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current)
      }
    }
  }, [query])

  const clearResults = useCallback(() => {
    setResults([])
    setQuery("")
  }, [])

  return { query, setQuery, results, isSearching, clearResults }
}
