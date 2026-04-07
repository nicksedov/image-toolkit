import { useEffect, useState } from "react"
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { useFolderPatterns } from "@/hooks/useFolderPatterns"
import { batchDelete } from "@/api/endpoints"
import { useTranslation } from "@/i18n"
import type { BatchDeleteRule } from "@/types"

interface BatchDeduplicationModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess: (message: string) => void
  onError: (message: string) => void
  onComplete: () => void
}

export function BatchDeduplicationModal({
  open,
  onOpenChange,
  onSuccess,
  onError,
  onComplete,
}: BatchDeduplicationModalProps) {
  const { patterns, isLoading, error, load } = useFolderPatterns()
  const [selectedFolders, setSelectedFolders] = useState<Record<string, string>>({})
  const [trashDir, setTrashDir] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)
  const { t } = useTranslation()

  useEffect(() => {
    if (open) {
      load()
      setSelectedFolders({})
    }
  }, [open, load])

  const handleApply = async () => {
    const rules: BatchDeleteRule[] = Object.entries(selectedFolders)
      .filter(([, folder]) => folder)
      .map(([patternId, keepFolder]) => ({ patternId, keepFolder }))

    if (rules.length === 0) {
      onError(t("batchDedup.errorNoRules"))
      return
    }

    if (!window.confirm(t("batchDedup.confirmApply", { count: rules.length }))) {
      return
    }

    setIsSubmitting(true)
    try {
      const result = await batchDelete({ rules, trashDir: trashDir.trim() })
      onOpenChange(false)
      let message: string
      if (result.failed > 0) {
        message = t("batchDedup.successWithFailed", { count: result.success, failed: result.failed })
      } else {
        message = t("batchDedup.success", { count: result.success })
      }
      onSuccess(message)
      onComplete()
    } catch (err) {
      onError(err instanceof Error ? err.message : t("batchDedup.errorFailed"))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("batchDedup.title")}</DialogTitle>
          <DialogDescription>
            {t("batchDedup.description")}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {isLoading ? (
            <div className="space-y-3">
              <Skeleton className="h-20 w-full" />
              <Skeleton className="h-20 w-full" />
              <Skeleton className="h-20 w-full" />
            </div>
          ) : error ? (
            <p className="text-sm text-destructive">{error}</p>
          ) : patterns.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-4">{t("batchDedup.noPatterns")}</p>
          ) : (
            <div className="max-h-80 overflow-y-auto space-y-3 pr-1">
              {patterns.map((pattern) => (
                <div key={pattern.id} className="rounded-md border p-3 space-y-2">
                  <div className="flex items-center gap-2">
                    <Badge variant="secondary" className="text-xs">
                      {t("batchDedup.groups", { count: pattern.duplicateCount })}
                    </Badge>
                    <Badge variant="outline" className="text-xs">
                      {t("batchDedup.files", { count: pattern.totalFiles })}
                    </Badge>
                  </div>
                  <RadioGroup
                    value={selectedFolders[pattern.id] || ""}
                    onValueChange={(value) =>
                      setSelectedFolders((prev) => ({ ...prev, [pattern.id]: value }))
                    }
                  >
                    {pattern.folders.map((folder) => (
                      <div key={folder} className="flex items-center gap-2">
                        <RadioGroupItem value={folder} id={`${pattern.id}-${folder}`} />
                        <Label
                          htmlFor={`${pattern.id}-${folder}`}
                          className="text-xs font-mono cursor-pointer truncate"
                        >
                          {folder}
                        </Label>
                      </div>
                    ))}
                  </RadioGroup>
                </div>
              ))}
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="batch-trash-dir">{t("batchDedup.trashDir")}</Label>
            <Input
              id="batch-trash-dir"
              placeholder={t("batchDedup.trashPlaceholder")}
              value={trashDir}
              onChange={(e) => setTrashDir(e.target.value)}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>{t("common.cancel")}</Button>
          <Button variant="destructive" onClick={handleApply} disabled={isSubmitting || isLoading}>
            {isSubmitting ? t("batchDedup.applying") : t("batchDedup.applyRules")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
