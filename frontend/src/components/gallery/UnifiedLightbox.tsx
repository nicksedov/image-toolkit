import { useCallback, useEffect, useState } from "react"
import { Sparkles, Info, ScanText } from "lucide-react"
import { useTranslation } from "@/i18n"
import { LightboxDialog } from "./lightbox/LightboxDialog"
import { buildImageUrl } from "@/utils/buildImageUrl"
import { OcrImagePanel } from "./lightbox/OcrImagePanel"
import { OcrResultPanel } from "./lightbox/OcrResultPanel"
import { ChatPanel } from "./lightbox/ChatPanel"
import { useOcrState } from "@/hooks/useOcrState"
import { useImageDimensions } from "@/hooks/useImageDimensions"
import { useFileExport } from "@/hooks/useFileExport"
import { useChatAgent } from "@/hooks/useChatAgent"
import { useImageMetadata } from "@/hooks/useImageMetadata"
import { Skeleton } from "@/components/ui/skeleton"
import { Button } from "@/components/ui/button"
import { Camera, MapPin, MapPinPlus, Image as ImageIcon, Pencil } from "lucide-react"
import { GeoSearchForm } from "./GeoSearchForm"
import type { ImageMetadataDTO } from "@/types"

export type LightboxMode = "ai" | "exif" | "ocr"

interface UnifiedLightboxProps {
  imagePath: string | null
  initialMode?: LightboxMode
  onClose: () => void
  showGeoForm?: boolean
  onShowGeoFormChange?: (show: boolean) => void
}

const TAB_CONFIG: { mode: LightboxMode; labelKey: "lightbox.tab.ai" | "lightbox.tab.exif" | "lightbox.tab.ocr"; icon: typeof Sparkles }[] = [
  { mode: "ai", labelKey: "lightbox.tab.ai", icon: Sparkles },
  { mode: "exif", labelKey: "lightbox.tab.exif", icon: Info },
  { mode: "ocr", labelKey: "lightbox.tab.ocr", icon: ScanText },
]

export function UnifiedLightbox({
  imagePath,
  initialMode = "ai",
  onClose,
  showGeoForm,
  onShowGeoFormChange,
}: UnifiedLightboxProps) {
  const { t, language } = useTranslation()
  const [activeMode, setActiveMode] = useState<LightboxMode>(initialMode)
  const [internalShowGeoForm, setInternalShowGeoForm] = useState(false)
  const isGeoFormVisible = showGeoForm ?? internalShowGeoForm

  const handleShowGeoForm = useCallback((show: boolean) => {
    if (onShowGeoFormChange) {
      onShowGeoFormChange(show)
    } else {
      setInternalShowGeoForm(show)
    }
  }, [onShowGeoFormChange])

  // Reset mode when lightbox opens
  useEffect(() => {
    if (imagePath) {
      setActiveMode(initialMode)
      setInternalShowGeoForm(false)
    }
  }, [imagePath, initialMode])

  // OCR state
  const { ocrData, llmData, loading: ocrLoading, recognizing, resetState: resetOcr, handleRecognize } = useOcrState(
    activeMode === "ocr" ? imagePath : null
  )
  const isTextDocument = ocrData?.isTextDocument ?? false
  const ocrImageUrl = imagePath
    ? isTextDocument && ocrData?.angle !== undefined
      ? buildImageUrl(imagePath, "/api/ocr-image", { angle: ocrData.angle })
      : buildImageUrl(imagePath, "/api/image")
    : ""
  const { imageRef, displayDimensions, imageLoaded, handleImageLoad } = useImageDimensions(ocrImageUrl)
  const { handleSaveMd, handleSaveHtml } = useFileExport(llmData?.markdownContent, imagePath)

  // AI Chat state
  const {
    conversation,
    conversations,
    messages,
    isStreaming,
    error: chatError,
    tokenCount,
    maxTokens,
    isTokenLimitReached,
    currentImagePath,
    createNewConversation,
    loadConversation,
    loadConversations,
    removeConversation,
    sendMessage,
    abortStream,
    resetForImage,
  } = useChatAgent(language)

  // Reset conversation state and load/create when image changes
  useEffect(() => {
    if (!imagePath || activeMode !== "ai") return

    // Reset state for the image (clears conversation and messages)
    resetForImage(imagePath)

    // Load existing conversations for this image
    loadConversations(imagePath)
  }, [imagePath, activeMode, resetForImage, loadConversations])

  // Create new conversation if none exists for current image
  useEffect(() => {
    if (!imagePath || activeMode !== "ai") return
    if (currentImagePath !== imagePath) return // Wait for reset to complete
    if (conversation) return // Already have a conversation
    if (conversations.length > 0) return // User can select from history

    // No conversations for this image - create a new one
    createNewConversation(imagePath)
  }, [imagePath, activeMode, currentImagePath, conversation, conversations.length, createNewConversation])

  const handleNewConversation = useCallback(() => {
    createNewConversation(imagePath || undefined)
  }, [imagePath, createNewConversation])

  const handleDeleteConversation = useCallback(() => {
    if (conversation) {
      removeConversation(conversation.id)
    }
  }, [conversation, removeConversation])

  const handleLoadConversation = useCallback((id: number) => {
    loadConversation(id)
  }, [loadConversation])

  // EXIF metadata state
  const { metadata, isLoading: metadataLoading, reload: reloadMetadata } = useImageMetadata(
    activeMode === "exif" ? imagePath : null
  )

  const handleGpsSaved = useCallback(() => {
    reloadMetadata()
    handleShowGeoForm(false)
  }, [reloadMetadata, handleShowGeoForm])

  // URLs
  const standardImageUrl = imagePath ? buildImageUrl(imagePath, "/api/image") : ""

  const handleClose = useCallback(() => {
    abortStream()
    resetOcr()
    resetForImage("")
    setInternalShowGeoForm(false)
    onClose()
  }, [abortStream, resetOcr, resetForImage, onClose])

  const formatProcessingTime = (ms?: number) => {
    if (!ms) return ""
    if (ms < 1000) return t("llm_ocr.milliseconds", { ms })
    return t("llm_ocr.seconds", { seconds: (ms / 1000).toFixed(1) })
  }

  if (!imagePath) return null

  return (
    <LightboxDialog
      open={!!imagePath}
      onOpenChange={() => handleClose()}
      titleKey="lightbox.title"
      descriptionKey="lightbox.description"
    >
      <div className="flex h-full">
        {/* Left: Image panel */}
        {activeMode === "ocr" ? (
          <OcrImagePanel
            imageUrl={ocrImageUrl}
            ocrData={ocrData}
            isTextDocument={isTextDocument}
            loading={ocrLoading}
            imageRef={imageRef}
            displayDimensions={displayDimensions}
            imageLoaded={imageLoaded}
            handleImageLoad={handleImageLoad}
            className="flex-1 flex items-center justify-center p-8 relative h-full"
          />
        ) : (
          <div className="flex-1 flex items-center justify-center bg-black min-h-[300px] min-w-0 h-full">
            <img
              src={standardImageUrl}
              alt={t("lightbox.alt")}
              className="max-w-full max-h-full object-contain"
            />
          </div>
        )}

        {/* Right: Panel with mode tabs */}
        <div className="w-full md:w-[400px] lg:w-[450px] md:min-w-[320px] border-t md:border-t-0 md:border-l bg-card h-full shrink-0 flex flex-col">
          {/* Tab bar */}
          <div className="flex pt-2 shrink-0">
            {TAB_CONFIG.map(({ mode, labelKey, icon: Icon }) => (
              <button
                key={mode}
                type="button"
                className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium transition-colors ${
                  activeMode === mode
                    ? "text-primary border-b-2 border-primary"
                    : "text-muted-foreground hover:text-foreground"
                }`}
                onClick={() => setActiveMode(mode)}
              >
                <Icon className="h-3.5 w-3.5" />
                {t(labelKey)}
              </button>
            ))}
          </div>

          {/* Panel content */}
          <div className="flex-1 min-h-0 overflow-hidden flex flex-col">
            {activeMode === "ai" && (
              <ChatPanelContent
                messages={messages}
                isStreaming={isStreaming}
                error={chatError}
                hasConversation={conversation !== null}
                imagePath={imagePath}
                tokenCount={tokenCount}
                maxTokens={maxTokens}
                isTokenLimitReached={isTokenLimitReached}
                conversations={conversations}
                activeConversationId={conversation?.id}
                onSendMessage={sendMessage}
                onAbortStream={abortStream}
                onNewConversation={handleNewConversation}
                onDeleteConversation={handleDeleteConversation}
                onLoadConversation={handleLoadConversation}
              />
            )}
            {activeMode === "exif" && (
              <ExifPanelContent
                metadata={metadata}
                isLoading={metadataLoading}
                imagePath={imagePath}
                showGeoForm={isGeoFormVisible}
                onShowGeoForm={() => handleShowGeoForm(true)}
                onGpsSaved={handleGpsSaved}
              />
            )}
            {activeMode === "ocr" && (
              <OcrResultPanel
                imagePath={imagePath}
                ocrData={ocrData}
                llmData={llmData}
                recognizing={recognizing}
                onRecognize={handleRecognize}
                onSaveMd={handleSaveMd}
                onSaveHtml={handleSaveHtml}
                formatProcessingTime={formatProcessingTime}
                className="w-full bg-card p-4 h-full flex flex-col"
              />
            )}
          </div>
        </div>
      </div>
    </LightboxDialog>
  )
}

// Wrapper to adapt ChatPanel to the unified layout (removes its own border-l and width)
function ChatPanelContent(props: {
  messages: import("@/types").ChatMessage[]
  isStreaming: boolean
  error: string | null
  hasConversation: boolean
  imagePath: string | null
  tokenCount: number
  maxTokens: number
  isTokenLimitReached: boolean
  conversations: import("@/types").Conversation[]
  activeConversationId?: number
  onSendMessage: (content: string) => void
  onAbortStream: () => void
  onNewConversation: () => void
  onDeleteConversation: () => void
  onLoadConversation: (id: number) => void
}) {
  return (
    <ChatPanel
      {...props}
      className="w-full h-full flex flex-col bg-card"
    />
  )
}

// EXIF panel content (adapted from ImageLightbox's MetadataContent)
function ExifPanelContent({
  metadata,
  isLoading,
  imagePath,
  showGeoForm,
  onShowGeoForm,
  onGpsSaved,
}: {
  metadata: ImageMetadataDTO | null
  isLoading: boolean
  imagePath: string
  showGeoForm: boolean
  onShowGeoForm: () => void
  onGpsSaved: () => void
}) {
  const { t } = useTranslation()

  return (
    <div className="h-full overflow-y-auto p-4">
      <h3 className="text-sm font-semibold mb-3">{t("metadata.title")}</h3>
      {isLoading ? (
        <MetadataSkeleton />
      ) : metadata ? (
        <MetadataContent
          metadata={metadata}
          imagePath={imagePath}
          showGeoForm={showGeoForm}
          onShowGeoForm={onShowGeoForm}
          onGpsSaved={onGpsSaved}
        />
      ) : (
        <div className="space-y-4">
          <p className="text-xs text-muted-foreground">{t("metadata.noData")}</p>
          <div>
            <div className="flex items-center gap-1.5 mb-2">
              <MapPin className="h-3.5 w-3.5 text-muted-foreground" />
              <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("metadata.sectionLocation")}</span>
            </div>
            {showGeoForm ? (
              <GeoSearchForm imagePath={imagePath} onGpsSaved={onGpsSaved} />
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
      )}
    </div>
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

  const coordsLabel =
    metadata.hasGps && metadata.gpsLatitude != null && metadata.gpsLongitude != null
      ? `${metadata.gpsLatitude.toFixed(4)}\u00b0, ${metadata.gpsLongitude.toFixed(4)}\u00b0`
      : ""
  const locationFields = buildFields([
    [t("metadata.nameLocal"), metadata.nameLocal],
    [t("metadata.nameEng"), metadata.nameEng],
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
          <>
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
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="w-full text-xs mt-2"
              onClick={onShowGeoForm}
            >
              <Pencil className="h-3.5 w-3.5 mr-1.5" />
              {t("geo.editLocation")}
            </Button>
          </>
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
