import { useState, useEffect } from "react"
import { Skeleton } from "@/components/ui/skeleton"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Camera, MapPin, MapPinPlus, Info, Image as ImageIcon, Loader2, Check } from "lucide-react"
import { useTranslation } from "@/i18n"
import { useImageMetadata } from "@/hooks/useImageMetadata"
import { useGeocodeSearch } from "@/hooks/useGeocodeSearch"
import { updateImageGps, fetchLocationCandidates } from "@/api/endpoints"
import { LightboxDialog } from "./lightbox/LightboxDialog"
import { buildImageUrl } from "@/utils/buildImageUrl"
import type { ImageMetadataDTO, GeocodeSearchResult, LocationCandidate } from "@/types"

interface ImageLightboxProps {
  imagePath: string | null
  onClose: () => void
  showGeoForm?: boolean
  onShowGeoFormChange?: (show: boolean) => void
}

export function ImageLightbox({ imagePath, onClose, showGeoForm, onShowGeoFormChange }: ImageLightboxProps) {
  const { t } = useTranslation()
  const { metadata, isLoading, reload } = useImageMetadata(imagePath)

  if (!imagePath) return null

  const imageUrl = buildImageUrl(imagePath, "/api/image")

  const handleGpsSaved = () => {
    reload()
    onShowGeoFormChange?.(false)
  }

  return (
    <LightboxDialog
      open={!!imagePath}
      onOpenChange={() => {
        onClose()
        onShowGeoFormChange?.(false)
      }}
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
              <MetadataContent
                metadata={metadata}
                imagePath={imagePath}
                showGeoForm={showGeoForm ?? false}
                onShowGeoForm={() => onShowGeoFormChange?.(true)}
                onGpsSaved={handleGpsSaved}
              />
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

interface MetadataContentProps {
  metadata: ImageMetadataDTO
  imagePath: string
  showGeoForm: boolean
  onShowGeoForm: () => void
  onGpsSaved: () => void
}

function MetadataContent({ metadata, imagePath, showGeoForm, onShowGeoForm, onGpsSaved }: MetadataContentProps) {
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
    locationFields.length > 0 ||
    !metadata.hasGps

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

      {/* Location section */}
      <div>
        <div className="flex items-center gap-1.5 mb-2">
          <MapPin className="h-3.5 w-3.5 text-muted-foreground" />
          <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("metadata.sectionLocation")}</span>
        </div>

        {showGeoForm ? (
          <GeoSearchForm imagePath={imagePath} onGpsSaved={onGpsSaved} />
        ) : metadata.hasGps ? (
          <div className="space-y-1.5">
            {locationFields.map(([label, value]) => (
              <div key={label} className="flex justify-between items-baseline gap-2 text-xs">
                <span className="text-muted-foreground shrink-0">{label}</span>
                <span className="font-medium text-right truncate" title={value}>
                  {value}
                </span>
              </div>
            ))}
          </div>
        ) : (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="w-full text-xs"
            onClick={onShowGeoForm}
          >
            <MapPinPlus className="h-3.5 w-3.5 mr-1.5" />
            {t("geo.addLocation")}
          </Button>
        )}
      </div>
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

// --- GeoSearchForm ---

interface GeoSearchFormProps {
  imagePath: string
  onGpsSaved: () => void
}

interface SelectedLocation {
  lat: number
  lng: number
  label: string
}

function GeoSearchForm({ imagePath, onGpsSaved }: GeoSearchFormProps) {
  const { t } = useTranslation()
  const { query, setQuery, results, isSearching } = useGeocodeSearch()
  const [candidates, setCandidates] = useState<LocationCandidate[]>([])
  const [selected, setSelected] = useState<SelectedLocation | null>(null)
  const [isSaving, setIsSaving] = useState(false)
  const [saveStatus, setSaveStatus] = useState<"idle" | "success" | "error">("idle")

  // Load location candidates on mount
  useEffect(() => {
    fetchLocationCandidates(imagePath)
      .then((res) => {
        if (res.candidates.length > 0) {
          setCandidates(res.candidates)
        }
      })
      .catch(() => {
        // Silently ignore - candidates are optional
      })
  }, [imagePath])

  const handleSelectCandidate = (c: LocationCandidate) => {
    const label = [c.geoCity, c.geoCountry].filter(Boolean).join(", ")
    setSelected({ lat: c.lat, lng: c.lng, label })
    setQuery("")
  }

  const handleSelectSearchResult = (r: GeocodeSearchResult) => {
    setSelected({ lat: r.lat, lng: r.lon, label: r.displayName })
    setQuery("")
  }

  const handleSave = async () => {
    if (!selected) return
    setIsSaving(true)
    setSaveStatus("idle")
    try {
      await updateImageGps({ path: imagePath, lat: selected.lat, lng: selected.lng })
      setSaveStatus("success")
      setTimeout(() => onGpsSaved(), 600)
    } catch {
      setSaveStatus("error")
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="space-y-3 mt-2">
      {/* Location candidates */}
      {candidates.length > 0 && !selected && (
        <div>
          <p className="text-[10px] text-muted-foreground mb-1.5">{t("geo.suggestedLocations")}</p>
          <div className="flex flex-wrap gap-1">
            {candidates.map((c, i) => {
              const label = [c.geoCity, c.geoCountry].filter(Boolean).join(", ")
              return (
                <button
                  key={i}
                  type="button"
                  className="inline-flex items-center gap-1 px-2 py-1 rounded-md text-[10px] border bg-background hover:bg-accent transition-colors"
                  onClick={() => handleSelectCandidate(c)}
                  title={`${label} (${c.photoCount})`}
                >
                  {label || `${c.lat.toFixed(2)}, ${c.lng.toFixed(2)}`}
                  <span className="text-muted-foreground">({c.photoCount})</span>
                </button>
              )
            })}
          </div>
        </div>
      )}

      {/* Search input */}
      <div className="relative">
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("geo.searchPlaceholder")}
          className="h-8 text-xs"
        />
        {isSearching && (
          <Loader2 className="absolute right-2 top-1/2 -translate-y-1/2 h-3.5 w-3.5 animate-spin text-muted-foreground" />
        )}

        {/* Search results dropdown */}
        {results.length > 0 && !selected && (
          <div className="absolute z-10 top-full left-0 right-0 mt-1 max-h-48 overflow-y-auto rounded-md border bg-popover shadow-md">
            {results.map((r, i) => (
              <button
                key={i}
                type="button"
                className="w-full text-left px-2.5 py-1.5 text-xs hover:bg-accent transition-colors truncate"
                onClick={() => handleSelectSearchResult(r)}
                title={r.displayName}
              >
                {r.displayName}
              </button>
            ))}
          </div>
        )}

        {query.length >= 2 && results.length === 0 && !isSearching && !selected && (
          <div className="absolute z-10 top-full left-0 right-0 mt-1 rounded-md border bg-popover shadow-md px-2.5 py-2 text-xs text-muted-foreground">
            {t("geo.noResults")}
          </div>
        )}
      </div>

      {/* Selected location display */}
      {selected && (
        <div className="rounded-md border bg-accent/50 p-2">
          <p className="text-[10px] text-muted-foreground mb-0.5">{t("geo.selectedLocation")}</p>
          <p className="text-xs font-medium truncate" title={selected.label}>{selected.label}</p>
          <p className="text-[10px] text-muted-foreground mt-0.5">
            {selected.lat.toFixed(4)}&deg;, {selected.lng.toFixed(4)}&deg;
          </p>
        </div>
      )}

      {/* Save button */}
      <Button
        type="button"
        size="sm"
        className="w-full text-xs"
        disabled={!selected || isSaving}
        onClick={handleSave}
        variant={saveStatus === "success" ? "default" : saveStatus === "error" ? "destructive" : "default"}
      >
        {isSaving ? (
          <>
            <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" />
            {t("geo.savingLocation")}
          </>
        ) : saveStatus === "success" ? (
          <>
            <Check className="h-3.5 w-3.5 mr-1.5" />
            {t("geo.saveSuccess")}
          </>
        ) : saveStatus === "error" ? (
          t("geo.saveFailed")
        ) : (
          <>
            <MapPin className="h-3.5 w-3.5 mr-1.5" />
            {t("geo.saveLocation")}
          </>
        )}
      </Button>
    </div>
  )
}
