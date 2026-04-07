import { useState } from "react"
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { deleteFiles } from "@/api/endpoints"
import { useTranslation } from "@/i18n"

interface DeleteFilesModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  selectedPaths: string[]
  onSuccess: (message: string) => void
  onError: (message: string) => void
  onComplete: () => void
}

export function DeleteFilesModal({
  open,
  onOpenChange,
  selectedPaths,
  onSuccess,
  onError,
  onComplete,
}: DeleteFilesModalProps) {
  const [trashDir, setTrashDir] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)
  const { t } = useTranslation()

  const handleDelete = async () => {
    if (!trashDir.trim()) {
      if (!window.confirm(t("deleteFiles.confirmPermanent"))) {
        return
      }
    }

    setIsSubmitting(true)
    try {
      const result = await deleteFiles({
        filePaths: selectedPaths,
        trashDir: trashDir.trim(),
      })
      onOpenChange(false)
      let message: string
      if (result.failed > 0) {
        message = t("deleteFiles.successWithFailed", { count: result.success, failed: result.failed })
      } else {
        message = t("deleteFiles.success", { count: result.success })
      }
      onSuccess(message)
      onComplete()
    } catch (err) {
      onError(err instanceof Error ? err.message : t("deleteFiles.errorFailed"))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("deleteFiles.title")}</DialogTitle>
          <DialogDescription>
            {t("deleteFiles.description", { count: selectedPaths.length })}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="rounded-md bg-destructive/10 border border-destructive/20 p-3 text-sm text-destructive">
            {t("deleteFiles.warning")}
          </div>
          <div className="space-y-2">
            <Label htmlFor="delete-trash-dir">{t("deleteFiles.trashDir")}</Label>
            <Input
              id="delete-trash-dir"
              placeholder={t("deleteFiles.trashPlaceholder")}
              value={trashDir}
              onChange={(e) => setTrashDir(e.target.value)}
            />
          </div>
          <p className="text-sm text-muted-foreground">
            {t("deleteFiles.hint")}
          </p>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>{t("common.cancel")}</Button>
          <Button variant="destructive" onClick={handleDelete} disabled={isSubmitting}>
            {isSubmitting ? t("deleteFiles.deleting") : t("deleteFiles.button")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
