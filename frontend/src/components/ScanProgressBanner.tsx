import { Progress } from "@/components/ui/progress"
import type { ScanStatusResponse } from "@/types"
import { Loader2 } from "lucide-react"

interface ScanProgressBannerProps {
  status: ScanStatusResponse
}

export function ScanProgressBanner({ status }: ScanProgressBannerProps) {
  if (!status.scanning) return null

  return (
    <div className="rounded-lg border border-blue-200 bg-blue-50 p-4 space-y-2">
      <div className="flex items-center gap-2 text-sm font-medium text-blue-800">
        <Loader2 className="h-4 w-4 animate-spin" />
        Scanning in progress...
      </div>
      <Progress value={undefined} className="h-1.5" />
      <div className="flex items-center justify-between text-xs text-blue-600">
        <span className="truncate max-w-md">{status.progress}</span>
        <span>{status.filesProcessed} files processed</span>
      </div>
    </div>
  )
}
