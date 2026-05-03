import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog"
import { VisuallyHidden } from "@radix-ui/react-visually-hidden"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { X, Camera, MapPin, Info, Image as ImageIcon } from "lucide-react"
import { useTranslation } from "@/i18n"
import { useImageMetadata } from "@/hooks/useImageMetadata"
import type { ImageMetadataDTO } from "@/types"

interface ImageLightboxProps {
  imagePath: string | null
  onClose: () => void
}

const API_BASE_URL = import.meta.env.VITE_API_URL || ""

export function ImageLightbox({ imagePath, onClose }: ImageLightboxProps) {
  const { t } = useTranslation()
  const { metadata, isLoading } = useImageMetadata(imagePath)

  if (!imagePath) return null

  const imageUrl = `${API_BASE_URL}/api/image?path=${encodeURIComponent(imagePath)}`

  return (
    <Dialog open={!!imagePath} onOpenChange={() => onClose()}>
      <DialogContent className="max-w-[95vw] max-h-[90vh] p-0 overflow-hidden">
        <VisuallyHidden>
          <DialogTitle>{t("lightbox.title")}</DialogTitle>
          <DialogDescription>{t("lightbox.description")}</DialogDescription>
        </VisuallyHidden>
        <Button
          variant="ghost"
          size="sm"
          className="absolute right-2 top-2 z-10 h-8 w-8 p-0 bg-black/50 text-white hover:bg-black/70 rounded-full"
          onClick={onClose}
        >
          <X className="h-4 w-4" />
        </Button>
        <div className="flex flex-col md:flex-row h-full max-h-[85vh]">
          {/* Image area */}
          <div className="flex-1 flex items-center justify-center bg-black min-h-[300px] min-w-0">
            <img
              src={imageUrl}
              alt={t("lightbox.alt")}
              className="max-w-full max-h-[85vh] md:max-h-[85vh] object-contain"
            />
          </div>

          {/* Metadata panel */}
          <div className="w-full md:w-[300px] lg:w-[340px] md:min-w-[280px] border-t md:border-t-0 md:border-l bg-card overflow-y-auto max-h-[40vh] md:max-h-[85vh] shrink-0">
            <div className="p-4">
              <h3 className="text-sm font-semibold mb-3">{t("metadata.title")}</h3>
              {isLoading ? (
                <MetadataSkeleton />
              ) : metadata ? (
                <MetadataContent metadata={metadata} />
              ) : (
                <p className="text-xs text-muted-foreground">{t("metadata.noData")}</p>
              )}
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}

function MetadataSkeleton() {
  return (
    <div className="space-y-3">
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="flex justify-between">
          <Skeleton className="h-3 w-20" />
          <Skeleton className="h-3 w-24" />
        </div>
      ))}
    </div>
  )
}

function MetadataContent({ metadata }: { metadata: ImageMetadataDTO }) {
  const { t } = useTranslation()

  const imageFields = buildFields([
    [t("metadata.dimensions"), metadata.dimensions],
  ])

  const cameraFields = buildFields([
    [t("metadata.camera"), metadata.cameraModel],
    [t("metadata.lens"), metadata.lensModel],
    [t("metadata.iso"), metadata.iso ? String(metadata.iso) : ""],
    [t("metadata.aperture"), metadata.aperture],
    [t("metadata.shutterSpeed"), metadata.shutterSpeed],
    [t("metadata.focalLength"), metadata.focalLength],
    [t("metadata.dateTaken"), metadata.dateTaken],
  ])

  const technicalFields = buildFields([
    [t("metadata.colorSpace"), metadata.colorSpace],
    [t("metadata.software"), metadata.software],
    [t("metadata.orientation"), metadata.orientation ? String(metadata.orientation) : ""],
  ])

  const locationLabel = [metadata.geoCity, metadata.geoCountry].filter(Boolean).join(", ")
  const coordsLabel =
    metadata.hasGps && metadata.gpsLatitude != null && metadata.gpsLongitude != null
      ? `${metadata.gpsLatitude.toFixed(4)}\u00b0, ${metadata.gpsLongitude.toFixed(4)}\u00b0`
      : ""
  const locationFields = buildFields([
    [t("metadata.location"), locationLabel],
    [t("metadata.coordinates"), coordsLabel],
  ])

  const hasAnything =
    imageFields.length > 0 ||
    cameraFields.length > 0 ||
    technicalFields.length > 0 ||
    locationFields.length > 0

  if (!hasAnything) {
    return <p className="text-xs text-muted-foreground">{t("metadata.noData")}</p>
  }

  return (
    <div className="space-y-4">
      {imageFields.length > 0 && (
        <MetadataSection icon={<ImageIcon className="h-3.5 w-3.5" />} title={t("metadata.sectionImage")} fields={imageFields} />
      )}
      {cameraFields.length > 0 && (
        <MetadataSection icon={<Camera className="h-3.5 w-3.5" />} title={t("metadata.sectionCamera")} fields={cameraFields} />
      )}
      {technicalFields.length > 0 && (
        <MetadataSection icon={<Info className="h-3.5 w-3.5" />} title={t("metadata.sectionTechnical")} fields={technicalFields} />
      )}
      {locationFields.length > 0 && (
        <MetadataSection icon={<MapPin className="h-3.5 w-3.5" />} title={t("metadata.sectionLocation")} fields={locationFields} />
      )}
    </div>
  )
}

function MetadataSection({
  icon,
  title,
  fields,
}: {
  icon: React.ReactNode
  title: string
  fields: [string, string][]
}) {
  return (
    <div>
      <div className="flex items-center gap-1.5 mb-2">
        <span className="text-muted-foreground">{icon}</span>
        <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{title}</span>
      </div>
      <div className="space-y-1.5">
        {fields.map(([label, value]) => (
          <div key={label} className="flex justify-between items-baseline gap-2 text-xs">
            <span className="text-muted-foreground shrink-0">{label}</span>
            <span className="font-medium text-right truncate" title={value}>
              {value}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

/** Filters out entries with empty values */
function buildFields(entries: [string, string][]): [string, string][] {
  return entries.filter(([, value]) => value !== "" && value !== "0")
}
