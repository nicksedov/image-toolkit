import { useCallback, useEffect } from "react"
import { useTranslation } from "@/i18n"
import { LightboxDialog } from "./lightbox/LightboxDialog"
import { buildImageUrl } from "@/utils/buildImageUrl"
import { AiImagePanel } from "./lightbox/AiImagePanel"
import { ChatPanel } from "./lightbox/ChatPanel"
import { useChatAgent } from "@/hooks/useChatAgent"

interface AiLightboxProps {
  imagePath: string | null
  onClose: () => void
}

export function AiLightbox({ imagePath, onClose }: AiLightboxProps) {
  const { language } = useTranslation()
  const {
    conversation,
    messages,
    isStreaming,
    error,
    createNewConversation,
    removeConversation,
    sendMessage,
    abortStream,
  } = useChatAgent(language)

  const imageUrl = imagePath
    ? buildImageUrl(imagePath, "/api/image")
    : ""

  // Create conversation when lightbox opens
  useEffect(() => {
    if (imagePath && !conversation) {
      createNewConversation(imagePath)
    }
  }, [imagePath, conversation, createNewConversation])

  const handleNewConversation = useCallback(() => {
    createNewConversation(imagePath || undefined)
  }, [imagePath, createNewConversation])

  const handleDeleteConversation = useCallback(() => {
    if (conversation) {
      removeConversation(conversation.id)
    }
  }, [conversation, removeConversation])

  const handleClose = useCallback(() => {
    abortStream()
    onClose()
  }, [onClose, abortStream])

  return (
    <LightboxDialog
      open={imagePath !== null}
      onOpenChange={() => handleClose()}
      titleKey="ai.title"
      descriptionKey="ai.description"
    >
      <div className="flex h-full">
        <AiImagePanel imageUrl={imageUrl} />
        <ChatPanel
          messages={messages}
          isStreaming={isStreaming}
          error={error}
          hasConversation={conversation !== null}
          imagePath={imagePath}
          onSendMessage={sendMessage}
          onAbortStream={abortStream}
          onNewConversation={handleNewConversation}
          onDeleteConversation={handleDeleteConversation}
        />
      </div>
    </LightboxDialog>
  )
}
