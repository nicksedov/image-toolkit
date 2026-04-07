import { useCallback, useState } from "react"
import { fetchFolderPatterns } from "@/api/endpoints"
import type { FolderPattern } from "@/types"

export function useFolderPatterns() {
  const [patterns, setPatterns] = useState<FolderPattern[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setIsLoading(true)
    setError(null)
    try {
      const result = await fetchFolderPatterns()
      setPatterns(result.patterns)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load folder patterns")
    } finally {
      setIsLoading(false)
    }
  }, [])

  return { patterns, isLoading, error, load }
}
