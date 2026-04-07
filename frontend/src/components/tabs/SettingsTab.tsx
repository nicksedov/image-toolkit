import { useCallback } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { AddFolderForm } from "@/components/settings/AddFolderForm"
import { FolderList } from "@/components/settings/FolderList"
import { ScanProgressBanner } from "@/components/ScanProgressBanner"
import { useGalleryFolders } from "@/hooks/useGalleryFolders"
import { useScanStatus } from "@/hooks/useScanStatus"
import { triggerScan } from "@/api/endpoints"
import { RefreshCw } from "lucide-react"

interface SettingsTabProps {
  onFolderAdded: () => void
}

export function SettingsTab({ onFolderAdded }: SettingsTabProps) {
  const { folders, isLoading, add, remove, refetch } = useGalleryFolders()
  const { status, startPolling, setOnScanComplete } = useScanStatus()

  const handleAdd = useCallback(
    async (path: string) => {
      try {
        const result = await add(path)
        toast.success(result.message)
        if (result.scanStarted) {
          setOnScanComplete(() => {
            refetch()
            toast.success("Scan complete!")
          })
          startPolling()
        }
        onFolderAdded()
      } catch (err) {
        toast.error(err instanceof Error ? err.message : "Failed to add folder")
      }
    },
    [add, startPolling, setOnScanComplete, refetch, onFolderAdded]
  )

  const handleRemove = useCallback(
    async (id: number) => {
      try {
        const result = await remove(id)
        toast.success(`${result.message} (${result.filesRemoved} files removed)`)
      } catch (err) {
        toast.error(err instanceof Error ? err.message : "Failed to remove folder")
      }
    },
    [remove]
  )

  const handleRescanAll = useCallback(async () => {
    if (folders.length === 0) {
      toast.error("No folders in the gallery to scan")
      return
    }
    try {
      await triggerScan()
      toast.success("Rescan started")
      setOnScanComplete(() => {
        refetch()
        toast.success("Rescan complete!")
      })
      startPolling()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to start rescan")
    }
  }, [folders.length, startPolling, setOnScanComplete, refetch])

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold mb-1">Gallery</h2>
        <p className="text-sm text-muted-foreground">
          Manage the folders included in your image gallery. Adding a folder will
          automatically start scanning it for images.
        </p>
      </div>

      <AddFolderForm onAdd={handleAdd} disabled={status.scanning} />

      <ScanProgressBanner status={status} />

      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-muted-foreground">
          {folders.length} folder{folders.length !== 1 ? "s" : ""} in gallery
        </h3>
        <Button
          variant="outline"
          size="sm"
          onClick={handleRescanAll}
          disabled={status.scanning || folders.length === 0}
        >
          <RefreshCw className={`mr-1.5 h-3.5 w-3.5 ${status.scanning ? "animate-spin" : ""}`} />
          Rescan All
        </Button>
      </div>

      <FolderList
        folders={folders}
        onRemove={handleRemove}
        isLoading={isLoading}
      />
    </div>
  )
}
