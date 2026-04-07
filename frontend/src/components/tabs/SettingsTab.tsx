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
import { useTranslation } from "@/i18n"

interface SettingsTabProps {
  onFolderAdded: () => void
}

export function SettingsTab({ onFolderAdded }: SettingsTabProps) {
  const { folders, isLoading, add, remove, refetch } = useGalleryFolders()
  const { status, startPolling, setOnScanComplete } = useScanStatus()
  const { t } = useTranslation()

  const handleAdd = useCallback(
    async (path: string) => {
      try {
        const result = await add(path)
        toast.success(result.message)
        if (result.scanStarted) {
          setOnScanComplete(() => {
            refetch()
            toast.success(t("settings.toastScanComplete"))
          })
          startPolling()
        }
        onFolderAdded()
      } catch (err) {
        toast.error(err instanceof Error ? err.message : t("settings.toastAddFailed"))
      }
    },
    [add, startPolling, setOnScanComplete, refetch, onFolderAdded, t]
  )

  const handleRemove = useCallback(
    async (id: number) => {
      try {
        const result = await remove(id)
        toast.success(t("settings.toastFilesRemoved", { message: result.message, count: result.filesRemoved }))
      } catch (err) {
        toast.error(err instanceof Error ? err.message : t("settings.toastRemoveFailed"))
      }
    },
    [remove, t]
  )

  const handleRescanAll = useCallback(async () => {
    if (folders.length === 0) {
      toast.error(t("settings.toastNoFolders"))
      return
    }
    try {
      await triggerScan()
      toast.success(t("settings.toastRescanStarted"))
      setOnScanComplete(() => {
        refetch()
        toast.success(t("settings.toastRescanComplete"))
      })
      startPolling()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("settings.toastRescanFailed"))
    }
  }, [folders.length, startPolling, setOnScanComplete, refetch, t])

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold mb-1">{t("settings.title")}</h2>
        <p className="text-sm text-muted-foreground">
          {t("settings.description")}
        </p>
      </div>

      <AddFolderForm onAdd={handleAdd} disabled={status.scanning} />

      <ScanProgressBanner status={status} />

      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-muted-foreground">
          {folders.length === 1
            ? t("settings.folderCountOne", { count: folders.length })
            : t("settings.folderCount", { count: folders.length })}
        </h3>
        <Button
          variant="outline"
          size="sm"
          onClick={handleRescanAll}
          disabled={status.scanning || folders.length === 0}
        >
          <RefreshCw className={`mr-1.5 h-3.5 w-3.5 ${status.scanning ? "animate-spin" : ""}`} />
          {t("settings.rescanAll")}
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
