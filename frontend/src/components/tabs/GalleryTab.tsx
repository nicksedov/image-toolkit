import { useCallback, useEffect, useRef, useState } from "react"
import { Button } from "@/components/ui/button"
import { GalleryImageGrid } from "@/components/gallery/GalleryImageGrid"
import { GalleryImageList } from "@/components/gallery/GalleryImageList"
import { ImageLightbox } from "@/components/gallery/ImageLightbox"
import { useGalleryImages } from "@/hooks/useGalleryImages"
import { Skeleton } from "@/components/ui/skeleton"
import { Grid3X3, List, ImageIcon } from "lucide-react"
import type { GalleryImageDTO } from "@/types"

export function GalleryTab() {
  const [viewMode, setViewMode] = useState<"list" | "thumbnails">("thumbnails")
  const [selectedImage, setSelectedImage] = useState<string | null>(null)
  const { images, totalImages, hasMore, isLoading, error, initialized, loadMore, reset } =
    useGalleryImages(viewMode)

  const sentinelRef = useRef<HTMLDivElement>(null)

  // Load first page on mount
  useEffect(() => {
    if (!initialized && !isLoading) {
      loadMore()
    }
  }, [initialized, isLoading, loadMore])

  // Infinite scroll observer
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

  const handleViewModeChange = useCallback(
    (mode: "list" | "thumbnails") => {
      setViewMode(mode)
      reset(mode)
    },
    [reset]
  )

  const handleImageClick = useCallback((image: GalleryImageDTO) => {
    setSelectedImage(image.path)
  }, [])

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <ImageIcon className="h-5 w-5 text-muted-foreground" />
          <span className="text-sm text-muted-foreground">
            {totalImages.toLocaleString()} image{totalImages !== 1 ? "s" : ""} in gallery
          </span>
        </div>
        <div className="flex items-center gap-1 rounded-md border p-0.5">
          <Button
            variant={viewMode === "thumbnails" ? "default" : "ghost"}
            size="sm"
            className="h-7 px-2"
            onClick={() => handleViewModeChange("thumbnails")}
          >
            <Grid3X3 className="h-3.5 w-3.5 mr-1" />
            Thumbnails
          </Button>
          <Button
            variant={viewMode === "list" ? "default" : "ghost"}
            size="sm"
            className="h-7 px-2"
            onClick={() => handleViewModeChange("list")}
          >
            <List className="h-3.5 w-3.5 mr-1" />
            List
          </Button>
        </div>
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
            No images in the gallery
          </p>
          <p className="text-xs text-muted-foreground/70">
            Add folders in the Settings tab to start browsing images.
          </p>
        </div>
      ) : (
        <>
          {viewMode === "thumbnails" ? (
            <GalleryImageGrid images={images} onImageClick={handleImageClick} />
          ) : (
            <GalleryImageList images={images} onImageClick={handleImageClick} />
          )}

          {/* Infinite scroll sentinel */}
          <div ref={sentinelRef} className="h-4" />

          {isLoading && (
            <div className="flex justify-center py-4">
              <div className="text-sm text-muted-foreground">Loading more images...</div>
            </div>
          )}

          {!hasMore && images.length > 0 && (
            <div className="text-center text-xs text-muted-foreground py-4">
              All {totalImages.toLocaleString()} images loaded
            </div>
          )}
        </>
      )}

      <ImageLightbox
        imagePath={selectedImage}
        onClose={() => setSelectedImage(null)}
      />
    </div>
  )
}
