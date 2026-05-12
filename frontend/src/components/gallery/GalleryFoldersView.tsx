import { useEffect } from "react"
import { GalleryImageGrid } from "@/components/gallery/GalleryImageGrid"
import { useGalleryImages } from "@/hooks/useGalleryImages"
import { Skeleton } from "@/components/ui/skeleton"
import { ImageIcon } from "lucide-react"
import { useTranslation } from "@/i18n"
import { useIntersectionObserver } from "@/hooks/useIntersectionObserver"
import { PaginationFooter } from "@/components/ui/pagination-footer"
import { ViewHeader } from "@/components/ui/view-header"
import type { GalleryImageDTO } from "@/types"

interface GalleryFoldersViewProps {
  onImageClick: (image: GalleryImageDTO) => void
  onImageView?: (image: GalleryImageDTO) => void
  onImageOcr?: (image: GalleryImageDTO) => void
  onImageAi?: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO, removeThumbnail: () => void) => void
}

export function GalleryFoldersView({ onImageClick, onImageView, onImageOcr, onImageAi, onImageDownload, onImageDelete }: GalleryFoldersViewProps) {
  const { images, totalImages, hasMore, isLoading, error, initialized, loadMore, removeImage } =
    useGalleryImages("folders")
  const { t } = useTranslation()

  const sentinelRef = useIntersectionObserver({
    onIntersect: loadMore,
    enabled: hasMore && !isLoading,
    dependencies: [hasMore, isLoading, loadMore],
  })

  useEffect(() => {
    if (!initialized && !isLoading) {
      loadMore()
    }
  }, [initialized, isLoading, loadMore])

  return (
    <div className="space-y-4">
      <ViewHeader
        icon={ImageIcon}
        textKey={totalImages === 1 ? "gallery.imageCountOne" : "gallery.imageCount"}
        textValues={{ count: totalImages.toLocaleString() }}
      />

      {error && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
          {error}
        </div>
      )}

      {!initialized && isLoading ? (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-40 w-full rounded-lg" />
          ))}
        </div>
      ) : images.length === 0 && !isLoading ? (
        <div className="rounded-lg border border-dashed p-12 text-center">
          <ImageIcon className="mx-auto h-10 w-10 text-muted-foreground/50" />
          <p className="mt-2 text-sm font-medium text-muted-foreground">
            {t("gallery.empty")}
          </p>
          <p className="text-xs text-muted-foreground/70">
            {t("gallery.emptyHint")}
          </p>
        </div>
      ) : (
        <>
          <GalleryImageGrid
            images={images}
            onImageClick={onImageClick}
            onImageView={onImageView}
            onImageOcr={onImageOcr}
            onImageAi={onImageAi}
            onImageDownload={onImageDownload}
            onImageDelete={(image) => onImageDelete?.(image, () => removeImage(image.id))}
          />

          <div ref={sentinelRef} className="h-4" />

          <PaginationFooter
            isLoading={isLoading}
            hasMore={hasMore}
            totalCount={totalImages}
          />
        </>
      )}
    </div>
  )
}
