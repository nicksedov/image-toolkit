import { Skeleton } from "@/components/ui/skeleton"
import { Camera, MapPin, Info, Image as ImageIcon } from "lucide-react"
import { useTranslation } from "@/i18n"
import { useImageMetadata } from "@/hooks/useImageMetadata"
import { LightboxDialog } from "./lightbox/LightboxDialog"
import { buildImageUrl } from "@/utils/buildImageUrl"
import type { ImageMetadataDTO } from "@/types"

interface ImageLightboxProps {
  imagePath: string | null
  onClose: () => void
}

export function ImageLightbox({ imagePath, onClose }: ImageLightboxProps) {
  const { t } = useTranslation()
  const { metadata, isLoading } = useImageMetadata(imagePath)

  if (!imagePath) return null

  const imageUrl = buildImageUrl(imagePath, "/api/image")

  return (
    <LightboxDialog
      open={!!imagePath}
      onOpenChange={() => onClose()}
      titleKey="lightbox.title"
      descriptionKey="lightbox.description"
    >
      <div className="flex flex-col md:flex-row h-full">
        <div className="flex-1 flex items-center justify-center bg-black min-h-[300px] min-w-0 h-full">
          <img
            src={imageUrl}
            alt={t("lightbox.alt")}
            className="max-w-full max-h-full object-contain"
          />
        </div>

        <div className="w-full md:w-[300px] lg:w-[340px] md:min-w-[280px] border-t md:border-t-0 md:border-l bg-card overflow-y-auto shrink-0 h-full">
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
    </LightboxDialog>
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

function buildFields(entries: [string, string][]): [string, string][] {
  return entries.filter(([, value]) => value !== "" && value !== "0")
}
