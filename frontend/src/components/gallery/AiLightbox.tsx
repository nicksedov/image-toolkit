import { useCallback, useState } from "react"
import { useTranslation } from "@/i18n"
import { LightboxDialog } from "./lightbox/LightboxDialog"
import { buildImageUrl } from "@/utils/buildImageUrl"
import { AiImagePanel } from "./lightbox/AiImagePanel"
import { AiActionPanel } from "./lightbox/AiActionPanel"
import { executeAiAction } from "@/api/endpoints"
import type { AiActionType, AiActionResponse } from "@/types"

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

  const imageUrl = imagePath
    ? buildImageUrl(imagePath, "/api/image")
    : ""

  const handleAction = useCallback(async (action: AiActionType, question?: string) => {
    if (!imagePath) return

    setLoading(true)
    setCurrentAction(action)
    setResult(null)
    setError(null)

    try {
      const response = await executeAiAction({
        imagePath,
        action,
        question,
        language,
      })

      if (response.success) {
        setResult(response)
      } else {
        setError(response.error || t("ai.actionFailed"))
      }
    } catch (err) {
      console.error("AI action failed:", err)
      setError(err instanceof Error ? err.message : t("ai.actionFailed"))
    } finally {
      setLoading(false)
    }
  }, [imagePath, t, language])

  const handleClose = useCallback(() => {
    setCurrentAction(null)
    setResult(null)
    setError(null)
    setLoading(false)
    onClose()
  }, [onClose])

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
