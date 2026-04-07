import { FileImage } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { GalleryImageDTO } from "@/types"

interface GalleryImageListProps {
  images: GalleryImageDTO[]
  onImageClick: (image: GalleryImageDTO) => void
}

export function GalleryImageList({ images, onImageClick }: GalleryImageListProps) {
  const { t } = useTranslation()

  return (
    <div className="rounded-lg border">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b bg-muted/50">
            <th className="p-2 text-left font-medium w-10"></th>
            <th className="p-2 text-left font-medium">{t("galleryList.fileName")}</th>
            <th className="p-2 text-left font-medium hidden md:table-cell">{t("galleryList.directory")}</th>
            <th className="p-2 text-right font-medium">{t("galleryList.size")}</th>
            <th className="p-2 text-right font-medium hidden sm:table-cell">{t("galleryList.modified")}</th>
          </tr>
        </thead>
        <tbody>
          {images.map((image) => (
            <tr
              key={image.id}
              className="border-b last:border-0 hover:bg-muted/30 cursor-pointer transition-colors"
              onClick={() => onImageClick(image)}
            >
              <td className="p-2">
                <FileImage className="h-4 w-4 text-muted-foreground" />
              </td>
              <td className="p-2 font-mono text-xs truncate max-w-[200px]">
                {image.fileName}
              </td>
              <td className="p-2 text-xs text-muted-foreground truncate max-w-[300px] hidden md:table-cell">
                {image.dirPath}
              </td>
              <td className="p-2 text-xs text-right text-muted-foreground whitespace-nowrap">
                {image.sizeHuman}
              </td>
              <td className="p-2 text-xs text-right text-muted-foreground whitespace-nowrap hidden sm:table-cell">
                {image.modTime}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
