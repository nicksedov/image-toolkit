import { useState } from "react"
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { generateScript } from "@/api/endpoints"
import type { GenerateScriptRequest } from "@/types"

interface GenerateScriptModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  selectedPaths: string[]
  onSuccess: (message: string) => void
  onError: (message: string) => void
}

export function GenerateScriptModal({
  open,
  onOpenChange,
  selectedPaths,
  onSuccess,
  onError,
}: GenerateScriptModalProps) {
  const [scriptType, setScriptType] = useState<"windows" | "bash">("windows")
  const [outputDir, setOutputDir] = useState("")
  const [trashDir, setTrashDir] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)

  const handleGenerate = async () => {
    if (!outputDir.trim()) {
      onError("Please specify an output directory for the script.")
      return
    }

    setIsSubmitting(true)
    try {
      const req: GenerateScriptRequest = {
        filePaths: selectedPaths,
        outputDir: outputDir.trim(),
        trashDir: trashDir.trim(),
        scriptType,
      }
      const result = await generateScript(req)
      onOpenChange(false)
      onSuccess(`Script generated successfully! Saved to: ${result.scriptPath}`)
    } catch (err) {
      onError(err instanceof Error ? err.message : "Failed to generate script")
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Generate Removal Script</DialogTitle>
          <DialogDescription>
            Generate a script to move {selectedPaths.length} selected file(s) to a trash directory.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="script-type">Script type</Label>
            <Select value={scriptType} onValueChange={(v) => setScriptType(v as "windows" | "bash")}>
              <SelectTrigger id="script-type">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="windows">Windows (PowerShell .ps1)</SelectItem>
                <SelectItem value="bash">Linux/macOS (Bash .sh)</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="output-dir">Output directory for script</Label>
            <Input
              id="output-dir"
              placeholder="C:\path\to\output"
              value={outputDir}
              onChange={(e) => setOutputDir(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="trash-dir">Trash directory (where files will be moved)</Label>
            <Input
              id="trash-dir"
              placeholder="C:\path\to\trash (optional)"
              value={trashDir}
              onChange={(e) => setTrashDir(e.target.value)}
            />
          </div>
          <p className="text-sm text-muted-foreground">
            The script will move selected files to the trash directory.
            Review the script before running it.
          </p>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={handleGenerate} disabled={isSubmitting}>
            {isSubmitting ? "Generating..." : "Generate Script"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
