import { useTranslation } from "@/i18n"
import type { GalleryImageDTO } from "@/types"

interface GalleryImageGridProps {
  images: GalleryImageDTO[]
  onImageClick: (image: GalleryImageDTO) => void
}

export function GalleryImageGrid({ images, onImageClick }: GalleryImageGridProps) {
  const { t } = useTranslation()

  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-2">
      {images.map((image) => (
        <button
          key={image.id}
          className="group relative aspect-square overflow-hidden rounded-lg border bg-muted hover:ring-2 hover:ring-ring transition-all cursor-pointer"
          onClick={() => onImageClick(image)}
        >
          {image.thumbnail ? (
            <img
              src={image.thumbnail}
              alt={image.fileName}
              className="h-full w-full object-cover"
              loading="lazy"
            />
          ) : (
            <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
              {t("gallery.noPreview")}
            </div>
          )}
          <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 to-transparent p-2 opacity-0 group-hover:opacity-100 transition-opacity">
            <div className="text-xs text-white truncate">{image.fileName}</div>
            <div className="text-xs text-white/70">{image.sizeHuman}</div>
          </div>
        </button>
      ))}
    </div>
  )
}
