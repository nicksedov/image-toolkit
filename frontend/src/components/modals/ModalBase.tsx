import { useState } from "react"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import { useSettings } from "@/providers/useSettings"
import { useTranslation } from "@/i18n"

interface ModalBaseProps {
  /** Whether the dialog is open */
  open: boolean
  /** Callback when open state changes */
  onOpenChange: (open: boolean) => void
  /** Dialog title */
  title: string
  /** Dialog description */
  description?: string
  /** Whether to show trash toggle */
  showTrashOption?: boolean
  /** Custom submit label */
  submitLabel?: string
  /** Custom submitting label */
  submittingLabel?: string
  /** Custom warning message */
  warningMessage?: React.ReactNode
  /** Additional content between header and footer */
  children?: React.ReactNode
  /** Submit handler - return false to prevent closing */
  onSubmit: () => Promise<boolean | void>
  /** Whether currently submitting */
  isSubmitting?: boolean
  /** Variant for submit button */
  submitVariant?: "default" | "destructive" | "outline" | "secondary"
  /** Additional footer buttons */
  extraFooter?: React.ReactNode
  /** Dialog content className */
  contentClassName?: string
}

/**
 * Base modal component with common dialog structure, trash toggle, and submit handling.
 */
export function ModalBase({
  open,
  onOpenChange,
  title,
  description,
  showTrashOption = false,
  submitLabel,
  submittingLabel,
  warningMessage,
  children,
  onSubmit,
  isSubmitting = false,
  submitVariant = "destructive",
  extraFooter,
  contentClassName,
}: ModalBaseProps) {
  const [useTrash, setUseTrash] = useState(true)
  const { trashDir } = useSettings()
  const { t } = useTranslation()

  const handleSubmit = async () => {
    // Confirm permanent delete if trash disabled or not configured
    if (!useTrash || !trashDir) {
      if (!window.confirm(t("deleteFiles.confirmPermanent"))) {
        return
      }
    }

    await onSubmit()
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className={contentClassName}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description && <DialogDescription>{description}</DialogDescription>}
        </DialogHeader>

        <div className="space-y-4">
          {warningMessage && (
            <div className="rounded-md bg-destructive/10 border border-destructive/20 p-3 text-sm text-destructive">
              {warningMessage}
            </div>
          )}

          {children}

          {showTrashOption && (
            <>
              <div className="flex items-center gap-2">
                <Checkbox
                  id="modal-use-trash"
                  checked={useTrash}
                  onCheckedChange={(checked) => setUseTrash(checked === true)}
                />
                <Label htmlFor="modal-use-trash" className="text-sm cursor-pointer">
                  {t("deleteFiles.useTrash")}
                </Label>
              </div>
              {useTrash && !trashDir && (
                <p className="text-xs text-destructive">
                  {t("deleteFiles.trashNotConfigured")}
                </p>
              )}
            </>
          )}
        </div>

        <DialogFooter>
          {extraFooter}
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isSubmitting}>
            {t("common.cancel")}
          </Button>
          <Button
            variant={submitVariant}
            onClick={handleSubmit}
            disabled={isSubmitting}
          >
            {isSubmitting ? (submittingLabel ?? "Submitting...") : (submitLabel ?? "Submit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/**
 * Hook to manage submit state for modal operations.
 */
export function useModalSubmit() {
  const [isSubmitting, setIsSubmitting] = useState(false)

  const handleSubmit = async (onSubmit: () => Promise<boolean | void>) => {
    setIsSubmitting(true)
    try {
      return await onSubmit()
    } finally {
      setIsSubmitting(false)
    }
  }

  return { isSubmitting, handleSubmit }
}
