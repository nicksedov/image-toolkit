import { useState } from "react"
import { ImageLightbox } from "@/components/gallery/ImageLightbox"
import { ExifFoldersView } from "@/components/gallery/ExifFoldersView"
import type { GalleryImageDTO } from "@/types"

export function ExifTab() {
  const [selectedImagePath, setSelectedImagePath] = useState<string | null>(null)
  const [showGeoForm, setShowGeoForm] = useState(false)

  const handleImageClick = (image: GalleryImageDTO) => {
    setSelectedImagePath(image.path)
  }

  const handleAddGeo = (image: GalleryImageDTO) => {
    setSelectedImagePath(image.path)
    setShowGeoForm(true)
  }

  return (
    <>
      <ExifFoldersView
        onImageClick={handleImageClick}
        onImageView={(image) => {
          setSelectedImagePath(image.path)
        }}
        onImageOcr={(image) => {
          window.dispatchEvent(new CustomEvent("open-ocr", { detail: { image } }))
        }}
        onImageAi={(image) => {
          window.dispatchEvent(new CustomEvent("open-ai", { detail: { image } }))
        }}
        onImageDownload={(image) => {
          const link = document.createElement("a")
          link.href = `/api/image?path=${encodeURIComponent(image.path)}`
          link.download = image.fileName
          link.click()
        }}
        onImageDelete={(image, removeThumbnail) => {
          window.dispatchEvent(
            new CustomEvent("delete-image", {
              detail: { imagePath: image.path, removeThumbnail },
            })
          )
        }}
        onAddGeo={handleAddGeo}
      />

      <ImageLightbox
        imagePath={selectedImagePath}
        onClose={() => {
          setSelectedImagePath(null)
          setShowGeoForm(false)
        }}
        showGeoForm={showGeoForm}
        onShowGeoFormChange={setShowGeoForm}
      />
    </>
  )
}
