import { Dialog, DialogContent, DialogTitle, DialogDescription } from "@/components/ui/dialog"
import { VisuallyHidden } from "@radix-ui/react-visually-hidden"
import { Button } from "@/components/ui/button"
import { X } from "lucide-react"
import { useTheme } from "@/theme"

interface LightboxDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  titleKey: string
  descriptionKey: string
  children: React.ReactNode
}

export function LightboxDialog({ open, onOpenChange, titleKey, descriptionKey, children }: LightboxDialogProps) {
  const { theme } = useTheme()
  const isLight = theme.startsWith("light-")
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className={`max-w-[95vw] w-[95vw] h-[90vh] p-0 overflow-hidden flex flex-col ${isLight ? "bg-white/90" : "bg-black/90"}`}>
        <VisuallyHidden>
          <DialogTitle>{titleKey}</DialogTitle>
          <DialogDescription>{descriptionKey}</DialogDescription>
        </VisuallyHidden>
        <Button
          variant="ghost"
          size="sm"
          className="absolute right-2 top-2 z-10 h-8 w-8 p-0 bg-background/80 text-foreground hover:bg-muted/80 rounded-full"
          onClick={() => onOpenChange(false)}
        >
          <X className="h-4 w-4" />
        </Button>
        {children}
      </DialogContent>
    </Dialog>
  )
}
