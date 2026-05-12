import { useState } from "react"
import { useTranslation } from "@/i18n"
import { Button } from "@/components/ui/button"
import {
  Sparkles,
  FileText,
  Tags,
  ScanText,
  MessageSquare,
  Loader2,
  AlertCircle,
} from "lucide-react"
import type { AiActionType, AiActionResponse } from "@/types"
import { OcrMarkdownRenderer } from "./OcrMarkdownRenderer"

interface AiActionPanelProps {
  imagePath: string | null
  currentAction: AiActionType | null
  result: AiActionResponse | null
  error: string | null
  loading: boolean
  onAction: (action: AiActionType, question?: string) => void
}

export function AiActionPanel({
  imagePath,
  currentAction,
  result,
  error,
  loading,
  onAction,
}: AiActionPanelProps) {
  const { t } = useTranslation()
  const [question, setQuestion] = useState("")
  const [showQuestionInput, setShowQuestionInput] = useState(false)

  const handleActionClick = (action: AiActionType) => {
    if (action === "askQuestion") {
      setShowQuestionInput(true)
    } else {
      onAction(action)
    }
  }

  const handleSubmitQuestion = () => {
    if (question.trim()) {
      onAction("askQuestion", question.trim())
      setQuestion("")
      setShowQuestionInput(false)
    }
  }

  const handleCancelQuestion = () => {
    setQuestion("")
    setShowQuestionInput(false)
  }

  const renderResult = () => {
    if (!result) return null

    switch (result.action) {
      case "describe":
        return (
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-sm font-medium text-foreground">
              <FileText className="h-4 w-4" />
              <span>{t("ai.description_result")}</span>
            </div>
            <div className="p-3 rounded-lg bg-muted text-sm">
              <OcrMarkdownRenderer content={result.result || ""} />
            </div>
          </div>
        )

      case "tags":
        return (
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-sm font-medium text-foreground">
              <Tags className="h-4 w-4" />
              <span>{t("ai.tags")}</span>
            </div>
            <div className="flex flex-wrap gap-2">
              {result.tags?.map((tag, index) => (
                <span
                  key={index}
                  className="px-2 py-1 rounded-md bg-primary/10 text-primary text-xs font-medium"
                >
                  {tag}
                </span>
              ))}
            </div>
          </div>
        )

      case "recognizeText":
        return (
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-sm font-medium text-foreground">
              <ScanText className="h-4 w-4" />
              <span>{t("ai.textRecognized")}</span>
            </div>
            <div className="p-3 rounded-lg bg-muted text-sm">
              <OcrMarkdownRenderer content={result.result || ""} />
            </div>
          </div>
        )

      case "askQuestion":
        return (
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-sm font-medium text-foreground">
              <MessageSquare className="h-4 w-4" />
              <span>{t("ai.answer")}</span>
            </div>
            <div className="p-3 rounded-lg bg-muted text-sm">
              <OcrMarkdownRenderer content={result.result || ""} />
            </div>
          </div>
        )

      default:
        return null
    }
  }

  return (
    <div className="w-full md:w-[400px] lg:w-[450px] md:min-w-[350px] border-l bg-card max-h-[90vh] shrink-0 flex flex-col">
      <div className="p-4 flex-shrink-0">
        <h3 className="text-sm font-semibold mb-4 flex items-center gap-2">
          <Sparkles className="h-4 w-4" />
          {t("ai.title")}
        </h3>

        {/* Action buttons */}
        <div className="grid grid-cols-2 gap-2 mb-4">
          <Button
            variant="outline"
            size="sm"
            className="h-auto py-3 px-3 flex flex-col items-center gap-1.5"
            onClick={() => handleActionClick("describe")}
            disabled={loading || !imagePath}
          >
            <FileText className="h-5 w-5" />
            <span className="text-xs text-center">{t("ai.describeImage")}</span>
          </Button>

          <Button
            variant="outline"
            size="sm"
            className="h-auto py-3 px-3 flex flex-col items-center gap-1.5"
            onClick={() => handleActionClick("tags")}
            disabled={loading || !imagePath}
          >
            <Tags className="h-5 w-5" />
            <span className="text-xs text-center">{t("ai.generateTags")}</span>
          </Button>

          <Button
            variant="outline"
            size="sm"
            className="h-auto py-3 px-3 flex flex-col items-center gap-1.5"
            onClick={() => handleActionClick("recognizeText")}
            disabled={loading || !imagePath}
          >
            <ScanText className="h-5 w-5" />
            <span className="text-xs text-center">{t("ai.recognizeText")}</span>
          </Button>

          <Button
            variant="outline"
            size="sm"
            className="h-auto py-3 px-3 flex flex-col items-center gap-1.5"
            onClick={() => handleActionClick("askQuestion")}
            disabled={loading || !imagePath}
          >
            <MessageSquare className="h-5 w-5" />
            <span className="text-xs text-center">{t("ai.askQuestion")}</span>
          </Button>
        </div>

        {/* Question input for askQuestion action */}
        {showQuestionInput && (
          <div className="space-y-2 mb-4">
            <textarea
              placeholder={t("ai.questionPlaceholder" as any)}
              value={question}
              onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setQuestion(e.target.value)}
              className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 min-h-[80px]"
            />
            <div className="flex gap-2">
              <Button
                size="sm"
                onClick={handleSubmitQuestion}
                disabled={!question.trim() || loading}
                className="flex-1"
              >
                {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : t("ai.submitQuestion")}
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={handleCancelQuestion}
                disabled={loading}
              >
                {t("common.cancel")}
              </Button>
            </div>
          </div>
        )}

        {/* Loading state */}
        {loading && currentAction && (
          <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
            <Loader2 className="h-8 w-8 animate-spin mb-3" />
            <p className="text-sm">{t("ai.processing")}</p>
          </div>
        )}

        {/* Error state */}
        {error && (
          <div className="flex items-start gap-2 p-3 rounded-lg bg-destructive/10 text-destructive text-sm">
            <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
            <span>{error}</span>
          </div>
        )}

        {/* Not enabled warning */}
        {!imagePath && (
          <div className="flex items-start gap-2 p-3 rounded-lg bg-muted text-muted-foreground text-sm">
            <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
            <span>{t("ai.notEnabled")}</span>
          </div>
        )}
      </div>

      {/* Result - scrollable independently */}
      {result && !loading && (
        <div className="flex-1 min-h-0 overflow-y-auto border-t px-4 py-4">
          {renderResult()}
        </div>
      )}
    </div>
  )
}
