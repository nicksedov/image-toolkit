import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { fetchTrashInfo, cleanTrash, updateSettings, fetchOCRStatus } from "@/api/endpoints"
import { useSettings } from "@/providers/useSettings"
import { useAuth } from "@/providers/AuthProvider"
import { RefreshCw, Trash2, Shield, Loader2 } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { OCRStatus } from "@/types"

export function AdminSettingsTab() {
  const { trashDir, setTrashDir } = useSettings()
  const { user } = useAuth()
  const { t } = useTranslation()
  const isAdmin = user?.role === "admin"

  const [trashInput, setTrashInput] = useState(trashDir)
  const [trashFileCount, setTrashFileCount] = useState(0)
  const [trashTotalSize, setTrashTotalSize] = useState("")
  const [isCleaning, setIsCleaning] = useState(false)
  const [isSavingTrash, setIsSavingTrash] = useState(false)
  const [ocrStatus, setOcrStatus] = useState<OCRStatus | null>(null)
  const [isOcrLoading, setIsOcrLoading] = useState(false)

  const loadTrashInfo = useCallback(() => {
    fetchTrashInfo()
      .then((info) => {
        setTrashFileCount(info.fileCount)
        setTrashTotalSize(info.totalSizeHuman)
      })
      .catch(() => {
        setTrashFileCount(0)
        setTrashTotalSize("")
      })
  }, [])

  useEffect(() => {
    loadTrashInfo()
  }, [loadTrashInfo])

  const loadOCRStatus = useCallback(async () => {
    try {
      setIsOcrLoading(true)
      const response = await fetchOCRStatus()
      setOcrStatus(response.status)
    } catch {
      setOcrStatus(null)
    } finally {
      setIsOcrLoading(false)
    }
  }, [])

  useEffect(() => {
    if (isAdmin) {
      loadOCRStatus()
    }
  }, [isAdmin, loadOCRStatus])

  const handleSaveTrashDir = useCallback(async () => {
    setIsSavingTrash(true)
    try {
      const result = await updateSettings({ trashDir: trashInput.trim() })
      setTrashDir(result.trashDir)
      setTrashInput(result.trashDir)
      toast.success(t("trash.saved"))
      loadTrashInfo()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("trash.saveFailed"))
    } finally {
      setIsSavingTrash(false)
    }
  }, [trashInput, setTrashDir, loadTrashInfo, t])

  const handleCleanTrash = useCallback(async () => {
    if (trashFileCount === 0) return
    if (!window.confirm(t("trash.cleanConfirm", { count: trashFileCount }))) return

    setIsCleaning(true)
    try {
      const result = await cleanTrash()
      toast.success(t("trash.cleanSuccess", { deleted: result.deleted }))
      loadTrashInfo()
    } catch {
      toast.error(t("trash.cleanFailed"))
    } finally {
      setIsCleaning(false)
    }
  }, [trashFileCount, loadTrashInfo, t])

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold mb-1">{t("adminPanel.adminSettings")}</h2>
        <p className="text-sm text-muted-foreground">
          {t("adminPanel.adminSettingsDescription")}
        </p>
      </div>

      {/* Trash Settings - Admin Only */}
      {isAdmin && (
        <Card>
          <CardHeader>
            <CardTitle>{t("trash.title")}</CardTitle>
            <CardDescription>{t("trash.description")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="trash-dir-input">{t("trash.dirLabel")}</Label>
              <div className="flex gap-2">
                <Input
                  id="trash-dir-input"
                  placeholder={t("trash.dirPlaceholder")}
                  value={trashInput}
                  onChange={(e) => setTrashInput(e.target.value)}
                  className="flex-1"
                />
                <Button
                  onClick={handleSaveTrashDir}
                  disabled={isSavingTrash || trashInput === trashDir}
                  size="default"
                >
                  {isSavingTrash ? t("trash.saving") : t("trash.save")}
                </Button>
              </div>
            </div>

            <div className="flex items-center justify-between rounded-md border p-3">
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Trash2 className="h-4 w-4" />
                {!trashDir ? (
                  <span>{t("trash.notConfigured")}</span>
                ) : trashFileCount === 0 ? (
                  <span>{t("trash.empty")}</span>
                ) : (
                  <span>{t("trash.fileCountWithSize", { count: trashFileCount, size: trashTotalSize })}</span>
                )}
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={handleCleanTrash}
                disabled={isCleaning || trashFileCount === 0 || !trashDir}
              >
                {isCleaning ? t("trash.cleaning") : t("trash.cleanButton")}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* OCR Status - Admin Only */}
      {isAdmin && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Shield className="h-5 w-5" />
              {t("adminPanel.ocr.title")}
            </CardTitle>
            <CardDescription>{t("adminPanel.ocr.description")}</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex items-center justify-between">
              <div className="space-y-1">
                <div className="text-sm font-medium">{t("adminPanel.ocr.status")}</div>
                {isOcrLoading ? (
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    {t("common.loading")}
                  </div>
                ) : (
                  <div className="flex items-center gap-2">
                    <span
                      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                        ocrStatus?.enabled && ocrStatus?.health === "healthy"
                          ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
                          : ocrStatus?.error || (ocrStatus?.enabled && ocrStatus?.health !== "healthy")
                          ? "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200"
                          : "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200"
                      }`}
                    >
                      {ocrStatus?.enabled && ocrStatus?.health === "healthy"
                        ? t("adminPanel.ocr.statusHealthy")
                        : ocrStatus?.error || (ocrStatus?.enabled && ocrStatus?.health !== "healthy")
                        ? t("adminPanel.ocr.statusError")
                        : t("adminPanel.ocr.statusDisabled")}
                    </span>
                  </div>
                )}
              </div>
              <Button variant="outline" size="sm" onClick={loadOCRStatus} disabled={isOcrLoading}>
                {isOcrLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
