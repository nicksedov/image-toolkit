import { useEffect, useRef } from "react"
import { GalleryImageGrid } from "@/components/gallery/GalleryImageGrid"
import { useGalleryImages } from "@/hooks/useGalleryImages"
import { Skeleton } from "@/components/ui/skeleton"
import { ImageIcon } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { GalleryImageDTO } from "@/types"

interface GalleryFoldersViewProps {
  onImageClick: (image: GalleryImageDTO) => void
}

export function GalleryFoldersView({ onImageClick }: GalleryFoldersViewProps) {
  const { images, totalImages, hasMore, isLoading, error, initialized, loadMore } =
    useGalleryImages("folders")
  const { t } = useTranslation()

  const sentinelRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!initialized && !isLoading) {
      loadMore()
    }
  }, [initialized, isLoading, loadMore])

  useEffect(() => {
    const sentinel = sentinelRef.current
    if (!sentinel) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasMore && !isLoading) {
          loadMore()
        }
      },
      { threshold: 0.1 }
    )

    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [hasMore, isLoading, loadMore])

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <ImageIcon className="h-5 w-5 text-muted-foreground" />
        <span className="text-sm text-muted-foreground">
          {totalImages === 1
            ? t("gallery.imageCountOne", { count: totalImages.toLocaleString() })
            : t("gallery.imageCount", { count: totalImages.toLocaleString() })}
        </span>
      </div>

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
          <GalleryImageGrid images={images} onImageClick={onImageClick} />

          <div ref={sentinelRef} className="h-4" />

          {isLoading && (
            <div className="flex justify-center py-4">
              <div className="text-sm text-muted-foreground">{t("gallery.loadingMore")}</div>
            </div>
          )}

          {!hasMore && images.length > 0 && (
            <div className="text-center text-xs text-muted-foreground py-4">
              {t("gallery.allLoaded", { count: totalImages.toLocaleString() })}
            </div>
          )}
        </>
      )}
    </div>
  )
}
