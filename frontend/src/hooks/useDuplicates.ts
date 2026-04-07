import { useCallback, useEffect, useState } from "react"
import { fetchDuplicates } from "@/api/endpoints"
import type { DuplicatesResponse } from "@/types"

export function useDuplicates(page: number, pageSize: number) {
  const [data, setData] = useState<DuplicatesResponse | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setIsLoading(true)
    setError(null)
    try {
      const result = await fetchDuplicates(page, pageSize)
      setData(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load duplicates")
    } finally {
      setIsLoading(false)
    }
  }, [page, pageSize])

  useEffect(() => {
    load()
  }, [load])

  return { data, isLoading, error, refetch: load }
}
