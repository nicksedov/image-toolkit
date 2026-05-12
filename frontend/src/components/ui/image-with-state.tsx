import { useRef, useState } from "react"
import { ImageOff, Loader2 } from "lucide-react"
import { Skeleton } from "@/components/ui/skeleton"
import { useTranslation } from "@/i18n"

export type ImageLoadingMode = "skeleton" | "spinner" | "none"

export interface ImageWithStateProps {
  /** Image source URL */
  src: string
  /** Alt text for the image */
  alt?: string
  /** Loading indicator mode */
  loadingMode?: ImageLoadingMode
  /** Custom class for the container */
  containerClassName?: string
  /** Custom class for the image */
  imageClassName?: string
  /** Custom class for loading state */
  loadingClassName?: string
  /** Custom class for error state */
  errorClassName?: string
  /** Custom loading component */
  loadingComponent?: React.ReactNode
  /** Custom error component */
  errorComponent?: React.ReactNode
  /** Callback when image loads */
  onLoaded?: () => void
  /** Callback when image fails to load */
  onError?: () => void
  /** Show opacity transition on load */
  fadeOnLoad?: boolean
}

export function ImageWithState({
  src,
  alt = "Image",
  loadingMode = "skeleton",
  containerClassName = "flex-1 flex items-center justify-center bg-black min-h-[300px] min-w-0 relative",
  imageClassName = "max-w-full max-h-full object-contain",
  loadingClassName = "absolute inset-0 flex items-center justify-center",
  errorClassName = "absolute inset-0 flex flex-col items-center justify-center text-muted-foreground",
  loadingComponent,
  errorComponent,
  onLoaded,
  onError,
  fadeOnLoad = true,
}: ImageWithStateProps) {
  const { t } = useTranslation()
  const imageRef = useRef<HTMLImageElement>(null)
  const [imageLoaded, setImageLoaded] = useState(false)
  const [imageError, setImageError] = useState(false)

  const handleImageLoad = () => {
    setImageLoaded(true)
    setImageError(false)
    onLoaded?.()
  }

  const handleImageError = () => {
    setImageError(true)
    setImageLoaded(false)
    onError?.()
  }

  const renderLoading = () => {
    if (loadingComponent) return loadingComponent

    switch (loadingMode) {
      case "skeleton":
        return (
          <div className={loadingClassName}>
            <Skeleton className="w-32 h-32 rounded-lg" />
          </div>
        )
      case "spinner":
        return (
          <div className={loadingClassName}>
            <Loader2 className="h-8 w-8 animate-spin" />
          </div>
        )
      case "none":
      default:
        return null
    }
  }

  const renderError = () => {
    if (errorComponent) return errorComponent

    return (
      <div className={errorClassName}>
        <ImageOff className="h-16 w-16 mb-4 opacity-50" />
        <p className="text-sm">{t("common.imageLoadError") || "Failed to load image"}</p>
      </div>
    )
  }

  return (
    <div className={containerClassName}>
      {!imageLoaded && !imageError && renderLoading()}

      {imageError && renderError()}

      <img
        ref={imageRef}
        src={src}
        alt={alt}
        className={`${imageClassName} ${fadeOnLoad ? "transition-opacity duration-200" : ""} ${
          fadeOnLoad ? (imageLoaded ? "opacity-100" : "opacity-0") : ""
        }`}
        onLoad={handleImageLoad}
        onError={handleImageError}
      />
    </div>
  )
}
