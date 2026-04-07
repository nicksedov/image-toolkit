import {
  Dialog,
  DialogContent,
  DialogTitle,
} from "@/components/ui/dialog"
import { VisuallyHidden } from "@radix-ui/react-visually-hidden"
import { Button } from "@/components/ui/button"
import { X } from "lucide-react"

interface ImageLightboxProps {
  imagePath: string | null
  onClose: () => void
}

const API_BASE_URL = import.meta.env.VITE_API_URL || ""

export function ImageLightbox({ imagePath, onClose }: ImageLightboxProps) {
  if (!imagePath) return null

  const imageUrl = `${API_BASE_URL}/api/image?path=${encodeURIComponent(imagePath)}`

  return (
    <Dialog open={!!imagePath} onOpenChange={() => onClose()}>
      <DialogContent className="max-w-[90vw] max-h-[90vh] p-0 overflow-hidden">
        <VisuallyHidden>
          <DialogTitle>Image preview</DialogTitle>
        </VisuallyHidden>
        <Button
          variant="ghost"
          size="sm"
          className="absolute right-2 top-2 z-10 h-8 w-8 p-0 bg-black/50 text-white hover:bg-black/70 rounded-full"
          onClick={onClose}
        >
          <X className="h-4 w-4" />
        </Button>
        <div className="flex items-center justify-center bg-black min-h-[300px]">
          <img
            src={imageUrl}
            alt="Full size preview"
            className="max-w-full max-h-[85vh] object-contain"
          />
        </div>
      </DialogContent>
    </Dialog>
  )
}
