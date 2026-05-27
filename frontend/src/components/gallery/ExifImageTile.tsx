import { Download, Image as ImageIcon, ScanText, Trash2, Sparkles, CalendarX2, MapPinX } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { GalleryImageDTO } from "@/types"

interface ExifImageTileProps {
  image: GalleryImageDTO
  onClick: (image: GalleryImageDTO) => void
  onImageView?: (image: GalleryImageDTO) => void
  onImageOcr?: (image: GalleryImageDTO) => void
  onImageAi?: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO) => void
  onAddGeo?: (image: GalleryImageDTO) => void
}

export function ExifImageTile({
  image,
  onClick,
  onImageView,
  onImageOcr,
  onImageAi,
  onImageDownload,
  onImageDelete,
  onAddGeo,
}: ExifImageTileProps) {
  const { t } = useTranslation()

  return (
    <button
      className="group flex flex-col cursor-pointer"
      onClick={() => onClick(image)}
    >
      <div className="relative aspect-square overflow-hidden rounded-lg border bg-muted hover:ring-2 hover:ring-ring transition-all">
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

        {/* EXIF missing data indicators in top-left corner */}
        <div className="absolute top-1 left-1 flex gap-1">
          {image.missingDate && (
            <div
              className="flex items-center justify-center h-5 w-5 rounded bg-black/60 text-amber-400"
              title={t("exif.missingDate")}
            >
              <CalendarX2 className="h-3.5 w-3.5" />
            </div>
          )}
          {image.missingGps && (
            onAddGeo ? (
              <button
                type="button"
                className="flex items-center justify-center h-5 w-5 rounded bg-black/60 text-red-400 hover:bg-red-500/80 hover:text-white cursor-pointer transition-colors"
                title={t("exif.missingGps")}
                onClick={(e) => {
                  e.stopPropagation()
                  onAddGeo(image)
                }}
              >
                <MapPinX className="h-3.5 w-3.5" />
              </button>
            ) : (
              <div
                className="flex items-center justify-center h-5 w-5 rounded bg-black/60 text-red-400"
                title={t("exif.missingGps")}
              >
                <MapPinX className="h-3.5 w-3.5" />
              </div>
            )
          )}
        </div>

        {/* Overlay with action buttons */}
        <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 to-transparent p-2 opacity-0 group-hover:opacity-100 transition-opacity">
          <div className="grid grid-cols-3 gap-1 justify-items-center">
            {onImageDownload && (
              <button
                type="button"
                className="p-2 rounded-lg bg-white/10 hover:bg-white/20 text-white transition-colors"
                onClick={(e) => {
                  e.stopPropagation()
                  onImageDownload(image)
                }}
                title={t("gallery.overlay.download")}
              >
                <Download className="h-5 w-5" />
              </button>
            )}
            {onImageView && (
              <button
                type="button"
                className="p-2 rounded-lg bg-white/10 hover:bg-white/20 text-white transition-colors"
                onClick={(e) => {
                  e.stopPropagation()
                  onImageView(image)
                }}
                title={t("gallery.overlay.view")}
              >
                <ImageIcon className="h-5 w-5" />
              </button>
            )}
            {onImageOcr && (
              <button
                type="button"
                className="p-2 rounded-lg bg-white/10 hover:bg-white/20 text-white transition-colors"
                onClick={(e) => {
                  e.stopPropagation()
                  onImageOcr(image)
                }}
                title={t("gallery.overlay.ocr")}
              >
                <ScanText className="h-5 w-5" />
              </button>
            )}
            {onImageAi && (
              <button
                type="button"
                className="p-2 rounded-lg bg-white/10 hover:bg-white/20 text-white transition-colors"
                onClick={(e) => {
                  e.stopPropagation()
                  onImageAi(image)
                }}
                title={t("gallery.overlay.ai")}
              >
                <Sparkles className="h-5 w-5" />
              </button>
            )}
            {onImageDelete && (
              <button
                type="button"
                className="p-2 rounded-lg bg-red-500/20 hover:bg-red-500/40 text-white transition-colors"
                onClick={(e) => {
                  e.stopPropagation()
                  onImageDelete(image)
                }}
                title={t("gallery.overlay.delete")}
              >
                <Trash2 className="h-5 w-5" />
              </button>
            )}
          </div>
        </div>
      </div>
      <p className="text-[11px] text-muted-foreground truncate mt-1 px-0.5 w-full text-center" title={image.fileName}>
        {image.fileName}
      </p>
    </button>
  )
}
