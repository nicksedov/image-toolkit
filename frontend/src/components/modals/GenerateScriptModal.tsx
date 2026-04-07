import { useState } from "react"
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { generateScript } from "@/api/endpoints"
import { useTranslation } from "@/i18n"
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
  const { t } = useTranslation()

  const handleGenerate = async () => {
    if (!outputDir.trim()) {
      onError(t("generateScript.errorOutputDir"))
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
      onSuccess(t("generateScript.success", { path: result.scriptPath }))
    } catch (err) {
      onError(err instanceof Error ? err.message : t("generateScript.errorFailed"))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("generateScript.title")}</DialogTitle>
          <DialogDescription>
            {t("generateScript.description", { count: selectedPaths.length })}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="script-type">{t("generateScript.scriptType")}</Label>
            <Select value={scriptType} onValueChange={(v) => setScriptType(v as "windows" | "bash")}>
              <SelectTrigger id="script-type">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="windows">{t("generateScript.windows")}</SelectItem>
                <SelectItem value="bash">{t("generateScript.bash")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="output-dir">{t("generateScript.outputDir")}</Label>
            <Input
              id="output-dir"
              placeholder={t("generateScript.outputPlaceholder")}
              value={outputDir}
              onChange={(e) => setOutputDir(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="trash-dir">{t("generateScript.trashDir")}</Label>
            <Input
              id="trash-dir"
              placeholder={t("generateScript.trashPlaceholder")}
              value={trashDir}
              onChange={(e) => setTrashDir(e.target.value)}
            />
          </div>
          <p className="text-sm text-muted-foreground">
            {t("generateScript.hint")}
          </p>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>{t("common.cancel")}</Button>
          <Button onClick={handleGenerate} disabled={isSubmitting}>
            {isSubmitting ? t("generateScript.generating") : t("generateScript.button")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
