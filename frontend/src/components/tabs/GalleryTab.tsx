import { useCallback, useState } from "react"
import { GalleryFoldersView } from "@/components/gallery/GalleryFoldersView"
import { GalleryCalendarView } from "@/components/gallery/GalleryCalendarView"
import { ImageLightbox } from "@/components/gallery/ImageLightbox"
import type { GalleryImageDTO } from "@/types"

interface GalleryTabProps {
  galleryMode: "folders" | "calendar"
}

export function GalleryTab({ galleryMode }: GalleryTabProps) {
  const [selectedImage, setSelectedImage] = useState<string | null>(null)

  const handleImageClick = useCallback((image: GalleryImageDTO) => {
    setSelectedImage(image.path)
  }, [])

  return (
    <div className="space-y-4">
      {galleryMode === "folders" ? (
        <GalleryFoldersView onImageClick={handleImageClick} />
      ) : (
        <GalleryCalendarView onImageClick={handleImageClick} />
      )}

      <ImageLightbox
        imagePath={selectedImage}
        onClose={() => setSelectedImage(null)}
      />
    </div>
  )
}
