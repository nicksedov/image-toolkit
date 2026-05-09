import { Calendar } from "lucide-react"
import type { CalendarDateGroup } from "@/types"
import type { GalleryImageDTO } from "@/types"
import { ImageTile } from "./ImageTile"

interface CalendarImageGridProps {
  groups: CalendarDateGroup[]
  onImageClick: (image: GalleryImageDTO) => void
  onImageView?: (image: GalleryImageDTO) => void
  onImageOcr?: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO, removeThumbnail: () => void) => void
}

export function CalendarImageGrid({ groups, onImageClick, onImageView, onImageOcr, onImageDownload, onImageDelete }: CalendarImageGridProps) {
  const handleDelete = (image: GalleryImageDTO) => {
    const removeThumbnail = () => {
      // Find the group and remove the image from it
      for (const group of groups) {
        const idx = group.images.findIndex((img) => img.id === image.id)
        if (idx !== -1) {
          group.images.splice(idx, 1)
          break
        }
      }
      // Force re-render by triggering state update in parent
      // This is a workaround - ideally the parent would manage this
    }
    onImageDelete?.(image, removeThumbnail)
  }

  return (
    <div className="space-y-5">
      {groups.map((group) => (
        <div key={group.date} id={`date-group-${group.date}`} className="mb-6">
          <div className="flex items-center gap-2 mb-2 px-0.5">
            <Calendar className="h-4 w-4 text-muted-foreground shrink-0" />
            <span className="text-sm font-medium">{group.label}</span>
            <span className="text-xs text-muted-foreground shrink-0">
              ({group.imageCount})
            </span>
          </div>
          <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-7 xl:grid-cols-8 gap-1.5">
            {group.images.map((image) => (
              <ImageTile
                key={image.id}
                image={image}
                onClick={onImageClick}
                onImageView={onImageView}
                onImageOcr={onImageOcr}
                onImageDownload={onImageDownload}
                onImageDelete={handleDelete}
              />
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
