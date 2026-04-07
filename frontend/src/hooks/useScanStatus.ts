import { useCallback, useEffect, useRef, useState } from "react"
import { fetchScanStatus } from "@/api/endpoints"
import type { ScanStatusResponse } from "@/types"
import { SCAN_POLL_INTERVAL } from "@/lib/constants"

export function useScanStatus() {
  const [status, setStatus] = useState<ScanStatusResponse>({
    scanning: false,
    progress: "",
    filesProcessed: 0,
  })
  const [isPolling, setIsPolling] = useState(false)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const wasScanning = useRef(false)
  const onScanCompleteRef = useRef<(() => void) | null>(null)

  const poll = useCallback(async () => {
    try {
      const result = await fetchScanStatus()
      setStatus(result)

      if (wasScanning.current && !result.scanning) {
        // Scan just finished
        onScanCompleteRef.current?.()
      }
      wasScanning.current = result.scanning
    } catch {
      // ignore polling errors
    }
  }, [])

  const startPolling = useCallback(() => {
    if (intervalRef.current) return
    setIsPolling(true)
    wasScanning.current = true
    poll()
    intervalRef.current = setInterval(poll, SCAN_POLL_INTERVAL)
  }, [poll])

  const stopPolling = useCallback(() => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current)
      intervalRef.current = null
    }
    setIsPolling(false)
  }, [])

  const setOnScanComplete = useCallback((callback: () => void) => {
    onScanCompleteRef.current = callback
  }, [])

  // Auto-stop polling when scan finishes
  useEffect(() => {
    if (isPolling && !status.scanning && wasScanning.current === false) {
      // Give a brief delay before stopping
      const timeout = setTimeout(stopPolling, 2000)
      return () => clearTimeout(timeout)
    }
  }, [isPolling, status.scanning, stopPolling])

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
      }
    }
  }, [])

  return {
    status,
    isPolling,
    startPolling,
    stopPolling,
    setOnScanComplete,
  }
}
