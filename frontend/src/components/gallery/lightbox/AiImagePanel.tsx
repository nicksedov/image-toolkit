import { useRef, useState } from "react"
import { Skeleton } from "@/components/ui/skeleton"
import { ImageOff } from "lucide-react"
import { useTranslation } from "@/i18n"

interface AiImagePanelProps {
  imageUrl: string
}

export function AiImagePanel({ imageUrl }: AiImagePanelProps) {
  const { t } = useTranslation()
  const imageRef = useRef<HTMLImageElement>(null)
  const [imageLoaded, setImageLoaded] = useState(false)
  const [imageError, setImageError] = useState(false)

  const handleImageLoad = () => {
    setImageLoaded(true)
    setImageError(false)
  }

  const handleImageError = () => {
    setImageError(true)
    setImageLoaded(false)
  }

  return (
    <div className="flex-1 flex items-center justify-center bg-black min-h-[300px] min-w-0 relative">
      {!imageLoaded && !imageError && (
        <div className="absolute inset-0 flex items-center justify-center">
          <Skeleton className="w-32 h-32 rounded-lg" />
        </div>
      )}

      {imageError && (
        <div className="absolute inset-0 flex flex-col items-center justify-center text-muted-foreground">
          <ImageOff className="h-16 w-16 mb-4 opacity-50" />
          <p className="text-sm">Failed to load image</p>
        </div>
      )}

      <img
        ref={imageRef}
        src={imageUrl}
        alt={t("ai.title")}
        className={`max-w-full max-h-full object-contain transition-opacity duration-200 ${
          imageLoaded ? "opacity-100" : "opacity-0"
        }`}
        onLoad={handleImageLoad}
        onError={handleImageError}
      />
    </div>
  )
}
