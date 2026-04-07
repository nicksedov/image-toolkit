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
      onError("Please select at least one folder to keep.")
      return
    }

    if (!window.confirm(`This will apply ${rules.length} rule(s) to delete duplicate files. Continue?`)) {
      return
    }

    setIsSubmitting(true)
    try {
      const result = await batchDelete({ rules, trashDir: trashDir.trim() })
      onOpenChange(false)
      let message = `Successfully deleted ${result.success} file(s).`
      if (result.failed > 0) {
        message += ` Failed: ${result.failed}.`
      }
      onSuccess(message)
      onComplete()
    } catch (err) {
      onError(err instanceof Error ? err.message : "Failed to apply batch rules")
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Batch Deduplication</DialogTitle>
          <DialogDescription>
            Select which folder should keep the file for each pattern.
            Files in other folders will be deleted.
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
            <p className="text-sm text-muted-foreground text-center py-4">No folder patterns found.</p>
          ) : (
            <div className="max-h-80 overflow-y-auto space-y-3 pr-1">
              {patterns.map((pattern) => (
                <div key={pattern.id} className="rounded-md border p-3 space-y-2">
                  <div className="flex items-center gap-2">
                    <Badge variant="secondary" className="text-xs">
                      {pattern.duplicateCount} groups
                    </Badge>
                    <Badge variant="outline" className="text-xs">
                      {pattern.totalFiles} files
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
            <Label htmlFor="batch-trash-dir">Trash directory (optional)</Label>
            <Input
              id="batch-trash-dir"
              placeholder="C:\path\to\trash (leave empty to delete permanently)"
              value={trashDir}
              onChange={(e) => setTrashDir(e.target.value)}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button variant="destructive" onClick={handleApply} disabled={isSubmitting || isLoading}>
            {isSubmitting ? "Applying..." : "Apply Rules"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
