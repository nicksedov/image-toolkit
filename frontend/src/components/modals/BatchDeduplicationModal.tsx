import { useEffect, useState } from "react"
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Checkbox } from "@/components/ui/checkbox"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { useFolderPatterns } from "@/hooks/useFolderPatterns"
import { batchDelete } from "@/api/endpoints"
import { useSettings } from "@/providers/useSettings"
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
  const [useTrash, setUseTrash] = useState(true)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const { trashDir } = useSettings()
  const { t } = useTranslation()

  useEffect(() => {
    if (open) {
      load()
      setSelectedFolders({})
      setUseTrash(true)
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

    if (!useTrash || !trashDir) {
      if (!window.confirm(t("batchDedup.confirmPermanent"))) {
        return
      }
    } else {
      if (!window.confirm(t("batchDedup.confirmApply", { count: rules.length }))) {
        return
      }
    }

    setIsSubmitting(true)
    try {
      const result = await batchDelete({
        rules,
        trashDir: useTrash ? trashDir : "",
      })
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
      <DialogContent className="max-w-2xl max-h-[85vh] flex flex-col overflow-hidden">
        <DialogHeader className="flex-shrink-0">
          <DialogTitle>{t("batchDedup.title")}</DialogTitle>
          <DialogDescription>
            {t("batchDedup.description")}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 overflow-hidden flex-1 min-h-0">
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
            <div className="overflow-y-auto space-y-3 pr-3 max-h-[50vh]">
              {patterns.map((pattern) => (
                <div key={pattern.id} className="rounded-md border p-3 space-y-2">
                  <div className="flex items-center gap-2 flex-wrap">
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
                      <div key={folder} className="flex items-center gap-2 min-w-0">
                        <RadioGroupItem value={folder} id={`${pattern.id}-${folder}`} className="flex-shrink-0" />
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
          <div className="flex items-center gap-2 flex-shrink-0">
            <Checkbox
              id="batch-use-trash"
              checked={useTrash}
              onCheckedChange={(checked) => setUseTrash(checked === true)}
            />
            <Label htmlFor="batch-use-trash" className="text-sm cursor-pointer">
              {t("batchDedup.useTrash")}
            </Label>
          </div>
          {useTrash && !trashDir && (
            <p className="text-xs text-destructive flex-shrink-0">
              {t("batchDedup.trashNotConfigured")}
            </p>
          )}
        </div>
        <DialogFooter className="flex-shrink-0">
          <Button variant="outline" onClick={() => onOpenChange(false)}>{t("common.cancel")}</Button>
          <Button variant="destructive" onClick={handleApply} disabled={isSubmitting || isLoading}>
            {isSubmitting ? t("batchDedup.applying") : t("batchDedup.applyRules")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
