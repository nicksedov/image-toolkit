import { useState } from "react"
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { deleteFiles } from "@/api/endpoints"

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

  const handleDelete = async () => {
    if (!trashDir.trim()) {
      if (!window.confirm("No trash directory specified. Files will be PERMANENTLY deleted. Continue?")) {
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
      let message = `Successfully deleted ${result.success} file(s).`
      if (result.failed > 0) {
        message += ` Failed: ${result.failed}.`
      }
      onSuccess(message)
      onComplete()
    } catch (err) {
      onError(err instanceof Error ? err.message : "Failed to delete files")
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete Selected Files</DialogTitle>
          <DialogDescription>
            This action will delete <strong>{selectedPaths.length}</strong> file(s).
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="rounded-md bg-destructive/10 border border-destructive/20 p-3 text-sm text-destructive">
            Warning: Deleted files cannot be easily recovered unless you specify a trash directory.
          </div>
          <div className="space-y-2">
            <Label htmlFor="delete-trash-dir">Trash directory (optional)</Label>
            <Input
              id="delete-trash-dir"
              placeholder="C:\path\to\trash (leave empty to delete permanently)"
              value={trashDir}
              onChange={(e) => setTrashDir(e.target.value)}
            />
          </div>
          <p className="text-sm text-muted-foreground">
            If a trash directory is specified, files will be moved there.
            Otherwise, files will be permanently deleted.
          </p>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button variant="destructive" onClick={handleDelete} disabled={isSubmitting}>
            {isSubmitting ? "Deleting..." : "Delete Files"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
