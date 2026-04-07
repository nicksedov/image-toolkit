import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { PAGE_SIZES } from "@/lib/constants"
import { RefreshCw, RotateCcw, Trash2, Layers } from "lucide-react"
import { useTranslation } from "@/i18n"

interface ToolbarProps {
  selectedCount: number
  pageSize: number
  onPageSizeChange: (size: number) => void
  onRescan: () => void
  onResetSelection: () => void
  onOpenDeleteFiles: () => void
  onOpenBatchDedup: () => void
  isScanning: boolean
}

export function Toolbar({
  selectedCount,
  pageSize,
  onPageSizeChange,
  onRescan,
  onResetSelection,
  onOpenDeleteFiles,
  onOpenBatchDedup,
  isScanning,
}: ToolbarProps) {
  const { t } = useTranslation()

  return (
    <div className="flex flex-wrap items-center gap-2 rounded-lg border bg-card p-3">
      <Button size="sm" onClick={onRescan} disabled={isScanning}>
        <RefreshCw className={`mr-1.5 h-3.5 w-3.5 ${isScanning ? "animate-spin" : ""}`} />
        {t("toolbar.rescan")}
      </Button>
      <Button size="sm" variant="outline" onClick={onResetSelection}>
        <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
        {t("toolbar.resetSelection")}
      </Button>
      <Button size="sm" variant="destructive" onClick={onOpenDeleteFiles} disabled={selectedCount === 0}>
        <Trash2 className="mr-1.5 h-3.5 w-3.5" />
        {t("toolbar.deleteSelected")}
      </Button>
      <Button size="sm" variant="outline" onClick={onOpenBatchDedup}>
        <Layers className="mr-1.5 h-3.5 w-3.5" />
        {t("toolbar.batchDedup")}
      </Button>

      <div className="ml-auto flex items-center gap-3">
        {selectedCount > 0 && (
          <Badge variant="secondary">
            {selectedCount === 1
              ? t("toolbar.filesSelectedOne", { count: selectedCount })
              : t("toolbar.filesSelected", { count: selectedCount })}
          </Badge>
        )}
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground whitespace-nowrap">{t("toolbar.groupsPerPage")}</span>
          <Select value={String(pageSize)} onValueChange={(v) => onPageSizeChange(Number(v))}>
            <SelectTrigger className="w-20 h-8 text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {PAGE_SIZES.map((size) => (
                <SelectItem key={size} value={String(size)}>{size}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
    </div>
  )
}
