import { useState, useRef, useEffect, useCallback } from "react"
import { useTranslation } from "@/i18n"
import { Button } from "@/components/ui/button"
import {
  Send,
  Square,
  Sparkles,
  ChevronDown,
  ChevronRight,
  Loader2,
  AlertCircle,
  Plus,
  Trash2,
  Image,
  Search,
  FileText,
  ScanText,
  Calendar,
} from "lucide-react"
import type { ChatMessage, ChatToolCallInfo } from "@/types"
import type { TranslationKey } from "@/i18n"
import { OcrMarkdownRenderer } from "./OcrMarkdownRenderer"
import { fetchThumbnail } from "@/api/endpoints"

interface ChatPanelProps {
  messages: ChatMessage[]
  isStreaming: boolean
  error: string | null
  hasConversation: boolean
  imagePath: string | null
  onSendMessage: (content: string) => void
  onAbortStream: () => void
  onNewConversation: () => void
  onDeleteConversation: () => void
}

interface Suggestion {
  icon: React.ReactNode
  labelKey: TranslationKey
  messageKey: TranslationKey
}

function ToolCallItem({ toolCall, isStreaming }: { toolCall: ChatToolCallInfo; isStreaming: boolean }) {
  const [expanded, setExpanded] = useState(false)
  const isRunning = isStreaming && !toolCall.result

  const toolDisplayName = toolCall.name
    .replace(/_/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase())

  return (
    <div className="border border-border/50 rounded-md text-xs overflow-hidden">
      <button
        type="button"
        className="flex items-center gap-1.5 w-full px-2 py-1.5 hover:bg-muted/50 transition-colors text-left"
        onClick={() => setExpanded(!expanded)}
      >
        {isRunning ? (
          <Loader2 className="h-3 w-3 animate-spin text-muted-foreground shrink-0" />
        ) : (
          <Sparkles className="h-3 w-3 text-primary shrink-0" />
        )}
        <span className="font-medium truncate">{toolDisplayName}</span>
        <span className="ml-auto shrink-0">
          {expanded ? (
            <ChevronDown className="h-3 w-3" />
          ) : (
            <ChevronRight className="h-3 w-3" />
          )}
        </span>
      </button>
      {expanded && toolCall.result && (
        <div className="border-t border-border/50 px-2 py-1.5 max-h-40 overflow-y-auto bg-muted/30">
          <pre className="whitespace-pre-wrap text-xs font-mono text-muted-foreground">
            {toolCall.result.length > 500
              ? toolCall.result.slice(0, 500) + "..."
              : toolCall.result}
          </pre>
        </div>
      )}
    </div>
  )
}

function MessageBubble({ message, isStreaming, imagePaths }: { message: ChatMessage; isStreaming: boolean; imagePaths: string[] }) {
  const isUser = message.role === "user"
  const isAssistant = message.role === "assistant"

  if (!isUser && !isAssistant) return null

  return (
    <div className={`flex ${isUser ? "justify-end" : "justify-start"} mb-3`}>
      <div
        className={`max-w-[85%] rounded-lg px-3 py-2 ${
          isUser
            ? "bg-primary text-primary-foreground"
            : "bg-muted text-foreground"
        }`}
      >
        {/* Tool calls */}
        {message.toolCalls && message.toolCalls.length > 0 && (
          <div className="space-y-1 mb-2">
            {message.toolCalls.map((tc, i) => (
              <ToolCallItem key={i} toolCall={tc} isStreaming={isStreaming} />
            ))}
          </div>
        )}

        {/* Message content */}
        {message.content && (
          <div className="text-sm">
            {isAssistant ? (
              <OcrMarkdownRenderer content={message.content} />
            ) : (
              <p className="whitespace-pre-wrap">{message.content}</p>
            )}
          </div>
        )}

        {/* Image thumbnails inline with the response */}
        {imagePaths.length > 0 && <ImageThumbnails paths={imagePaths} />}

        {/* Streaming indicator for empty assistant message */}
        {isAssistant && !message.content && isStreaming && (
          <div className="flex items-center gap-1.5 text-muted-foreground text-sm">
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
            <span className="italic text-xs">Thinking...</span>
          </div>
        )}
      </div>
    </div>
  )
}

// Extract image paths from tool call results (search tools return paths)
function extractImagePaths(message: ChatMessage): string[] {
  if (!message.toolCalls) return []
  const paths: string[] = []
  for (const tc of message.toolCalls) {
    if (tc.name.startsWith("search_") && tc.result) {
      try {
        const parsed = JSON.parse(tc.result)
        if (Array.isArray(parsed)) {
          for (const item of parsed) {
            if (item.path) paths.push(item.path)
          }
        } else if (parsed.images && Array.isArray(parsed.images)) {
          for (const item of parsed.images) {
            if (item.path) paths.push(item.path)
          }
        }
      } catch {
        // Not JSON, skip
      }
    }
  }
  return paths
}

function ThumbnailItem({ path }: { path: string }) {
  const [src, setSrc] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    fetchThumbnail(path)
      .then((res) => {
        if (!cancelled) setSrc(res.thumbnail)
      })
      .catch(() => {
        // leave empty on failure
      })
    return () => { cancelled = true }
  }, [path])

  if (!src) {
    return (
      <div className="w-full h-full flex items-center justify-center">
        <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <img
      src={src}
      alt={path.split("/").pop() || path}
      className="w-full h-full object-cover"
    />
  )
}

function ImageThumbnails({ paths }: { paths: string[] }) {
  if (paths.length === 0) return null
  const display = paths.slice(0, 8)

  return (
    <div className="flex flex-wrap gap-1.5 mt-2">
      {display.map((path, i) => (
        <div
          key={i}
          className="w-14 h-14 rounded overflow-hidden border border-border/50 bg-muted/30 shrink-0"
        >
          <ThumbnailItem path={path} />
        </div>
      ))}
      {paths.length > 8 && (
        <div className="w-14 h-14 rounded border border-border/50 bg-muted/30 flex items-center justify-center text-xs text-muted-foreground shrink-0">
          +{paths.length - 8}
        </div>
      )}
    </div>
  )
}

export function ChatPanel({
  messages,
  isStreaming,
  error,
  hasConversation,
  imagePath,
  onSendMessage,
  onAbortStream,
  onNewConversation,
  onDeleteConversation,
}: ChatPanelProps) {
  const { t } = useTranslation()
  const [input, setInput] = useState("")
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const messagesContainerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  const suggestions: Suggestion[] = [
    {
      icon: <FileText className="h-3.5 w-3.5" />,
      labelKey: "chat.suggest_describe",
      messageKey: "chat.suggest_describe_msg",
    },
    {
      icon: <ScanText className="h-3.5 w-3.5" />,
      labelKey: "chat.suggest_text",
      messageKey: "chat.suggest_text_msg",
    },
    {
      icon: <Search className="h-3.5 w-3.5" />,
      labelKey: "chat.suggest_similar",
      messageKey: "chat.suggest_similar_msg",
    },
    {
      icon: <Calendar className="h-3.5 w-3.5" />,
      labelKey: "chat.suggest_date",
      messageKey: "chat.suggest_date_msg",
    },
  ]

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    if (messagesEndRef.current) {
      messagesEndRef.current.scrollIntoView({ behavior: "smooth" })
    }
  }, [messages, isStreaming])

  // Focus input when conversation starts
  useEffect(() => {
    if (hasConversation && inputRef.current) {
      inputRef.current.focus()
    }
  }, [hasConversation])

  const handleSend = useCallback(() => {
    const trimmed = input.trim()
    if (!trimmed || isStreaming) return
    onSendMessage(trimmed)
    setInput("")
  }, [input, isStreaming, onSendMessage])

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault()
        handleSend()
      }
    },
    [handleSend],
  )

  const handleSuggestionClick = useCallback(
    (suggestion: Suggestion) => {
      if (isStreaming) return
      onSendMessage(t(suggestion.messageKey))
    },
    [isStreaming, onSendMessage, t],
  )

  // Auto-resize textarea
  const handleInputChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setInput(e.target.value)
    const el = e.target
    el.style.height = "auto"
    el.style.height = Math.min(el.scrollHeight, 120) + "px"
  }, [])

  return (
    <div className="w-full md:w-[400px] lg:w-[450px] md:min-w-[350px] border-l bg-card h-full shrink-0 flex flex-col">
      {/* Header */}
      <div className="flex items-center gap-2 px-4 py-3 border-b shrink-0">
        <div className="flex items-center gap-2">
          <Sparkles className="h-4 w-4 text-primary shrink-0" />
          <span className="text-sm font-semibold">{t("chat.title")}</span>
          {hasConversation && (
            <div className="flex gap-1 ml-1">
              <Button
                variant="ghost"
                size="sm"
                className="h-7 w-7 p-0"
                onClick={onNewConversation}
                title={t("chat.new_conversation")}
              >
                <Plus className="h-3.5 w-3.5" />
              </Button>
              <Button
                variant="ghost"
                size="sm"
                className="h-7 w-7 p-0 text-destructive hover:text-destructive"
                onClick={onDeleteConversation}
                title={t("common.delete")}
              >
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            </div>
          )}
        </div>
      </div>

      {/* Messages area */}
      <div
        ref={messagesContainerRef}
        className="flex-1 min-h-0 overflow-y-auto px-4 py-3"
      >
        {!hasConversation && (
          <div className="flex flex-col items-center justify-center h-full text-center text-muted-foreground">
            <Image className="h-10 w-10 mb-3 opacity-50" />
            <p className="text-sm font-medium mb-1">{t("chat.welcome_title")}</p>
            <p className="text-xs">{t("chat.welcome_description")}</p>
          </div>
        )}

        {hasConversation && messages.length === 0 && !isStreaming && (
          <div className="flex flex-col items-center justify-center h-full text-center text-muted-foreground">
            <Sparkles className="h-8 w-8 mb-3 opacity-50" />
            <p className="text-sm">{t("chat.start_hint")}</p>
          </div>
        )}

        {messages.map((msg, idx) => {
          const imagePaths = extractImagePaths(msg)
          return (
            <MessageBubble
              key={msg.id || idx}
              message={msg}
              isStreaming={isStreaming && idx === messages.length - 1}
              imagePaths={imagePaths}
            />
          )
        })}

        {/* Typing indicator */}
        {isStreaming && messages.length > 0 && (
          <div className="flex items-center gap-1 text-muted-foreground text-xs ml-1">
            <span className="animate-pulse">●</span>
            <span className="animate-pulse" style={{ animationDelay: "0.2s" }}>●</span>
            <span className="animate-pulse" style={{ animationDelay: "0.4s" }}>●</span>
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Suggestions — shown whenever user input is expected */}
      {hasConversation && !isStreaming && imagePath && (
        <div className="px-4 pb-2 flex flex-wrap gap-1.5 shrink-0">
          {suggestions.map((s, i) => (
            <button
              key={i}
              type="button"
              className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-full border border-border/70 text-xs text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
              onClick={() => handleSuggestionClick(s)}
            >
              {s.icon}
              <span>{t(s.labelKey)}</span>
            </button>
          ))}
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="flex items-start gap-2 px-4 py-2 bg-destructive/10 text-destructive text-xs shrink-0">
          <AlertCircle className="h-3.5 w-3.5 shrink-0 mt-0.5" />
          <span className="flex-1">{error}</span>
        </div>
      )}

      {/* Input area */}
      <div className="border-t px-4 py-3 shrink-0">
        {!hasConversation ? (
          <Button
            className="w-full"
            onClick={onNewConversation}
            disabled={!imagePath}
          >
            <Sparkles className="h-4 w-4 mr-2" />
            {t("chat.start_button")}
          </Button>
        ) : (
          <div className="flex items-end gap-2">
            <textarea
              ref={inputRef}
              value={input}
              onChange={handleInputChange}
              onKeyDown={handleKeyDown}
              placeholder={t("chat.placeholder")}
              rows={1}
              className="flex-1 resize-none rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
              disabled={isStreaming}
              style={{ maxHeight: 120 }}
            />
            {isStreaming ? (
              <Button
                size="sm"
                variant="destructive"
                className="shrink-0 h-9 w-9 p-0"
                onClick={onAbortStream}
              >
                <Square className="h-3.5 w-3.5" />
              </Button>
            ) : (
              <Button
                size="sm"
                className="shrink-0 h-9 w-9 p-0"
                onClick={handleSend}
                disabled={!input.trim()}
              >
                <Send className="h-3.5 w-3.5" />
              </Button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
