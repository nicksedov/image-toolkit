import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import { Folder, Trash2, FileImage } from "lucide-react"
import type { GalleryFolderDTO } from "@/types"

interface FolderListProps {
  folders: GalleryFolderDTO[]
  onRemove: (id: number) => Promise<void>
  isLoading: boolean
}

export function FolderList({ folders, onRemove, isLoading }: FolderListProps) {
  const [removingId, setRemovingId] = useState<number | null>(null)
  const [confirmFolder, setConfirmFolder] = useState<GalleryFolderDTO | null>(null)

  const handleRemove = async () => {
    if (!confirmFolder) return
    setRemovingId(confirmFolder.id)
    try {
      await onRemove(confirmFolder.id)
    } finally {
      setRemovingId(null)
      setConfirmFolder(null)
    }
  }

  if (isLoading) {
    return (
      <div className="text-sm text-muted-foreground py-8 text-center">
        Loading gallery folders...
      </div>
    )
  }

  if (folders.length === 0) {
    return (
      <div className="rounded-lg border border-dashed p-8 text-center">
        <Folder className="mx-auto h-10 w-10 text-muted-foreground/50" />
        <p className="mt-2 text-sm font-medium text-muted-foreground">
          No folders in the gallery
        </p>
        <p className="text-xs text-muted-foreground/70">
          Add a folder above to start scanning images.
        </p>
      </div>
    )
  }

  return (
    <>
      <div className="space-y-2">
        {folders.map((folder) => (
          <Card key={folder.id} className="flex items-center gap-3 p-3">
            <Folder className="h-5 w-5 text-blue-500 shrink-0" />
            <div className="flex-1 min-w-0">
              <div className="font-mono text-sm truncate">{folder.path}</div>
              <div className="flex items-center gap-3 text-xs text-muted-foreground mt-0.5">
                <span className="flex items-center gap-1">
                  <FileImage className="h-3 w-3" />
                  {folder.fileCount} files
                </span>
                <span>Added: {folder.createdAt}</span>
              </div>
            </div>
            <Button
              variant="ghost"
              size="sm"
              className="text-destructive hover:text-destructive hover:bg-destructive/10 shrink-0"
              onClick={() => setConfirmFolder(folder)}
              disabled={removingId === folder.id}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </Card>
        ))}
      </div>

      <Dialog open={!!confirmFolder} onOpenChange={() => setConfirmFolder(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remove Folder</DialogTitle>
            <DialogDescription>
              Are you sure you want to remove this folder from the gallery?
              All indexed files from this folder will be removed from the database.
              The actual files on disk will NOT be deleted.
            </DialogDescription>
          </DialogHeader>
          {confirmFolder && (
            <div className="rounded-md bg-muted p-3 font-mono text-sm truncate">
              {confirmFolder.path}
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmFolder(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleRemove}
              disabled={removingId !== null}
            >
              {removingId !== null ? "Removing..." : "Remove"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
