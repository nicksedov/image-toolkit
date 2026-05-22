import { useCallback, useState, useRef } from "react"
import { useTranslation } from "@/i18n"
import { LightboxDialog } from "./lightbox/LightboxDialog"
import { buildImageUrl } from "@/utils/buildImageUrl"
import { AiImagePanel } from "./lightbox/AiImagePanel"
import { AiActionPanel } from "./lightbox/AiActionPanel"
import { startAiAction, fetchAiActionStatus } from "@/api/endpoints"
import type { AiActionType, AiActionResponse, AiActionStatusResponse } from "@/types"

interface AiLightboxProps {
  imagePath: string | null
  onClose: () => void
}

export function AiLightbox({ imagePath, onClose }: AiLightboxProps) {
  const { t, language } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [currentAction, setCurrentAction] = useState<AiActionType | null>(null)
  const [result, setResult] = useState<AiActionResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const imageUrl = imagePath
    ? buildImageUrl(imagePath, "/api/image")
    : ""

  // Convert AiActionStatusResponse to AiActionResponse format
  const convertStatusToResult = (status: AiActionStatusResponse): AiActionResponse => {
    return {
      success: status.status === "completed",
      action: status.action,
      result: status.result,
      tags: status.tags,
      error: status.error,
      provider: status.provider,
      model: status.model,
      processingTimeMs: status.processingTimeMs,
    }
  }

  // Stop polling
  const stopPolling = useCallback(() => {
    if (pollingRef.current) {
      clearInterval(pollingRef.current)
      pollingRef.current = null
    }
  }, [])

  // Start polling for task status
  const startPolling = useCallback((taskId: string) => {
    stopPolling()

    pollingRef.current = setInterval(async () => {
      try {
        const status = await fetchAiActionStatus(taskId)

        if (status.status === "completed") {
          stopPolling()
          setLoading(false)
          setResult(convertStatusToResult(status))
        } else if (status.status === "failed") {
          stopPolling()
          setLoading(false)
          setError(status.error || t("ai.actionFailed"))
        }
        // If still "processing", continue polling
      } catch (err) {
        stopPolling()
        setLoading(false)
        setError(err instanceof Error ? err.message : t("ai.actionFailed"))
      }
    }, 1000) // Poll every 1 second
  }, [stopPolling, t])

  const handleAction = useCallback(async (action: AiActionType, question?: string) => {
    if (!imagePath) return

    setLoading(true)
    setCurrentAction(action)
    setResult(null)
    setError(null)

    try {
      const startResponse = await startAiAction({
        imagePath,
        action,
        question,
        language,
      })

      // Start polling for result
      startPolling(startResponse.taskId)
    } catch (err) {
      console.error("AI action start failed:", err)
      setLoading(false)
      setError(err instanceof Error ? err.message : t("ai.actionFailed"))
    }
  }, [imagePath, t, language, startPolling])

  const handleClose = useCallback(() => {
    stopPolling()
    setCurrentAction(null)
    setResult(null)
    setError(null)
    setLoading(false)
    onClose()
  }, [onClose, stopPolling])

  return (
    <LightboxDialog
      open={imagePath !== null}
      onOpenChange={() => handleClose()}
      titleKey="ai.title"
      descriptionKey="ai.description"
    >
      <div className="flex h-full">
        <AiImagePanel imageUrl={imageUrl} />
        <AiActionPanel
          imagePath={imagePath}
          currentAction={currentAction}
          result={result}
          error={error}
          loading={loading}
          onAction={handleAction}
        />
      </div>
    </LightboxDialog>
  )
}
