import { useCallback, useState } from "react"
import { GalleryFoldersView } from "@/components/gallery/GalleryFoldersView"
import { GalleryCalendarView } from "@/components/gallery/GalleryCalendarView"
import { GalleryGeolocationView } from "@/components/gallery/GalleryGeolocationView"
import { ImageLightbox } from "@/components/gallery/ImageLightbox"
import { OcrLightbox } from "@/components/gallery/OcrLightbox"
import type { GalleryImageDTO } from "@/types"

interface GalleryTabProps {
  galleryMode: "folders" | "calendar" | "geolocation"
}

const API_BASE_URL = import.meta.env.VITE_API_URL || ""

export function GalleryTab({ galleryMode }: GalleryTabProps) {
  const [selectedImage, setSelectedImage] = useState<string | null>(null)
  const [ocrImage, setOcrImage] = useState<string | null>(null)

  const handleImageClick = useCallback((image: GalleryImageDTO) => {
    setSelectedImage(image.path)
  }, [])

  const handleImageView = useCallback((image: GalleryImageDTO) => {
    setSelectedImage(image.path)
  }, [])

  const handleImageOcr = useCallback((image: GalleryImageDTO) => {
    setOcrImage(image.path)
  }, [])

  const handleImageDownload = useCallback((image: GalleryImageDTO) => {
    const imageUrl = `${API_BASE_URL}/api/image?path=${encodeURIComponent(image.path)}`
    const a = document.createElement("a")
    a.href = imageUrl
    a.download = image.fileName
    a.click()
  }, [])

  return (
    <div className="space-y-4">
      {galleryMode === "folders" ? (
        <GalleryFoldersView
          onImageClick={handleImageClick}
          onImageView={handleImageView}
          onImageOcr={handleImageOcr}
          onImageDownload={handleImageDownload}
        />
      ) : galleryMode === "calendar" ? (
        <GalleryCalendarView
          onImageClick={handleImageClick}
          onImageView={handleImageView}
          onImageOcr={handleImageOcr}
          onImageDownload={handleImageDownload}
        />
      ) : (
        <GalleryGeolocationView
          onImageClick={handleImageClick}
          onImageView={handleImageView}
          onImageOcr={handleImageOcr}
          onImageDownload={handleImageDownload}
        />
      )}

      <ImageLightbox
        imagePath={selectedImage}
        onClose={() => setSelectedImage(null)}
      />

      <OcrLightbox
        imagePath={ocrImage}
        onClose={() => setOcrImage(null)}
      />
    </div>
  )
}
