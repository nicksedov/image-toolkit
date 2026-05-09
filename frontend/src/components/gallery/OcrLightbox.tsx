import { useCallback } from "react"
import { useTranslation } from "@/i18n"
import { LightboxDialog } from "./lightbox/LightboxDialog"
import { buildImageUrl } from "@/utils/buildImageUrl"
import { OcrImagePanel } from "./lightbox/OcrImagePanel"
import { OcrResultPanel } from "./lightbox/OcrResultPanel"
import { useOcrState } from "@/hooks/useOcrState"
import { useImageDimensions } from "@/hooks/useImageDimensions"
import { useFileExport } from "@/hooks/useFileExport"

interface OcrLightboxProps {
  imagePath: string | null
  onClose: () => void
}

export function OcrLightbox({ imagePath, onClose }: OcrLightboxProps) {
  const { t } = useTranslation()
  const { ocrData, llmData, loading, recognizing, resetState, handleRecognize } = useOcrState(imagePath)
  const { imageRef, displayDimensions, imageLoaded, handleImageLoad } = useImageDimensions()
  const { handleSaveMd, handleSaveHtml } = useFileExport(llmData?.markdownContent, imagePath)

  const handleClose = useCallback(() => {
    resetState()
    onClose()
  }, [resetState, onClose])

  const angle = ocrData?.angle || 0
  const imageUrl = imagePath
    ? buildImageUrl(imagePath, "/api/ocr-image", { angle })
    : ""

  const formatProcessingTime = (ms?: number) => {
    if (!ms) return ""
    if (ms < 1000) return t("llm_ocr.milliseconds", { ms })
    return t("llm_ocr.seconds", { seconds: (ms / 1000).toFixed(1) })
  }

  return (
    <LightboxDialog
      open={imagePath !== null}
      onOpenChange={() => handleClose()}
      titleKey="lightbox.ocrTitle"
      descriptionKey="lightbox.ocrDescription"
    >
      <div className="flex h-full">
        <OcrImagePanel
          imageUrl={imageUrl}
          ocrData={ocrData}
          loading={loading}
          imageRef={imageRef}
          displayDimensions={displayDimensions}
          imageLoaded={imageLoaded}
          handleImageLoad={handleImageLoad}
        />
        <OcrResultPanel
          imagePath={imagePath}
          ocrData={ocrData}
          llmData={llmData}
          recognizing={recognizing}
          onRecognize={handleRecognize}
          onSaveMd={handleSaveMd}
          onSaveHtml={handleSaveHtml}
          formatProcessingTime={formatProcessingTime}
        />
      </div>
    </LightboxDialog>
  )
}
