import { Loader2, Wand2, Download } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { OcrDataResponse, LlmOcrDataResponse } from "@/types"
import { OcrMarkdownRenderer } from "./OcrMarkdownRenderer"

interface OcrResultPanelProps {
  imagePath: string | null
  ocrData: OcrDataResponse | null
  llmData: LlmOcrDataResponse | null
  recognizing: boolean
  onRecognize: () => void
  onSaveMd: () => void
  onSaveHtml: () => void
  formatProcessingTime: (ms?: number) => string
}

function detectLanguageFromOcr(ocrData: OcrDataResponse): string {
  let ruCount = 0
  let enCount = 0
  for (const box of ocrData.boxes) {
    for (const ch of box.word.toLowerCase()) {
      if (ch.charCodeAt(0) >= 0x0400 && ch.charCodeAt(0) <= 0x04FF) ruCount++
      if (ch.charCodeAt(0) >= 0x0061 && ch.charCodeAt(0) <= 0x007A) enCount++
    }
  }
  return ruCount > enCount ? "Русский" : "English"
}

export function OcrResultPanel({
  imagePath,
  ocrData,
  llmData,
  recognizing,
  onRecognize,
  onSaveMd,
  onSaveHtml,
  formatProcessingTime,
}: OcrResultPanelProps) {
  const { t } = useTranslation()

  if (recognizing) {
    return (
      <div className="w-[50%] bg-card border-l p-4 h-full flex flex-col">
        <div className="flex flex-col items-center justify-center h-full">
          <Loader2 className="h-12 w-12 animate-spin text-primary mb-4" />
          <p className="text-lg font-medium">{t("llm_ocr.recognizing")}</p>
        </div>
      </div>
    )
  }

  if (llmData?.found && llmData.success && llmData.markdownContent) {
    return (
      <div className="w-[50%] bg-card border-l p-4 h-full flex flex-col">
        <div className="flex-shrink-0 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-lg font-semibold">{t("llm_ocr.title")}</h3>
            <button
              onClick={onRecognize}
              className="flex items-center gap-2 px-3 py-1.5 text-sm bg-primary text-primary-foreground rounded hover:bg-primary/90"
            >
              <Wand2 className="h-4 w-4" />
              {t("llm_ocr.recognizeButton")}
            </button>
          </div>

          <div className="space-y-2 text-sm">
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t("llm_ocr.language")}:</span>
              <span className="font-medium">{llmData.language === "ru" ? "Русский" : "English"}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t("llm_ocr.provider")}:</span>
              <span className="font-medium">{llmData.provider}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t("llm_ocr.model")}:</span>
              <span className="font-medium">{llmData.model}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t("llm_ocr.processingTime")}:</span>
              <span className="font-medium">{formatProcessingTime(llmData.processingTimeMs)}</span>
            </div>
            {imagePath && (
              <div>
                <span className="text-muted-foreground">{t("llm_ocr.filePath")}:</span>
                <p className="text-xs break-all mt-1">{imagePath}</p>
              </div>
            )}
          </div>

          <div className="flex gap-2">
            <button
              onClick={onSaveMd}
              className="flex items-center gap-2 px-3 py-1.5 text-sm bg-primary/10 hover:bg-primary/20 text-primary rounded transition-colors"
            >
              <Download className="h-3.5 w-3.5" />
              Save as .md
            </button>
            <button
              onClick={onSaveHtml}
              className="flex items-center gap-2 px-3 py-1.5 text-sm bg-primary/10 hover:bg-primary/20 text-primary rounded transition-colors"
            >
              <Download className="h-3.5 w-3.5" />
              Save as .html
            </button>
          </div>
        </div>

        <div className="flex-1 mt-4 overflow-y-auto min-h-0">
          <div className="p-4 bg-muted rounded-lg markdown-body">
            <OcrMarkdownRenderer content={llmData.markdownContent} />
          </div>
        </div>
      </div>
    )
  }

  if (llmData?.error) {
    return (
      <div className="w-[50%] bg-card border-l p-4 h-full flex flex-col">
        <div className="space-y-4">
          <h3 className="text-lg font-semibold text-destructive">{t("llm_ocr.failed")}</h3>
          <p className="text-sm text-muted-foreground">{llmData.error}</p>
          <button
            onClick={onRecognize}
            className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded hover:bg-primary/90"
          >
            <Wand2 className="h-4 w-4" />
            {t("llm_ocr.recognizeButton")}
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="w-[50%] bg-card border-l p-4 h-full flex flex-col">
      <div className="space-y-4">
        <h3 className="text-lg font-semibold">{t("llm_ocr.title")}</h3>

        <div className="space-y-2 text-sm">
          {imagePath && (
            <div>
              <span className="text-muted-foreground">{t("llm_ocr.filePath")}:</span>
              <p className="text-xs break-all mt-1">{imagePath}</p>
            </div>
          )}
          {ocrData && (
            <div className="flex justify-between">
              <span className="text-muted-foreground">{t("llm_ocr.language")}:</span>
              <span className="font-medium">{detectLanguageFromOcr(ocrData)}</span>
            </div>
          )}
        </div>

        <button
          onClick={onRecognize}
          className="w-full flex items-center justify-center gap-2 px-4 py-3 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors"
        >
          <Wand2 className="h-5 w-5" />
          {t("llm_ocr.recognizeButton")}
        </button>

        <p className="text-xs text-muted-foreground text-center">
          {t("llm_ocr.description")}
        </p>
      </div>
    </div>
  )
}
