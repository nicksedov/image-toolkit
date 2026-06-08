import { useState, useCallback, useRef } from "react"
import type { Conversation, ChatMessage, SSEEvent } from "@/types"
import {
  createConversation,
  fetchConversations,
  deleteConversation,
  fetchConversationMessages,
  sendMessageStream,
} from "@/api/endpoints"

interface UseChatAgentReturn {
  conversation: Conversation | null
  conversations: Conversation[]
  messages: ChatMessage[]
  isStreaming: boolean
  error: string | null
  createNewConversation: (imagePath?: string) => Promise<void>
  loadConversation: (id: number) => Promise<void>
  loadConversations: () => Promise<void>
  removeConversation: (id: number) => Promise<void>
  sendMessage: (content: string) => void
  abortStream: () => void
}

export function useChatAgent(language: string = "en"): UseChatAgentReturn {
  const [conversation, setConversation] = useState<Conversation | null>(null)
  const [conversations, setConversations] = useState<Conversation[]>([])
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [isStreaming, setIsStreaming] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  const createNewConversation = useCallback(async (imagePath?: string) => {
    try {
      const conv = await createConversation({ imagePath, language })
      setConversation(conv)
      setMessages([])
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create conversation")
    }
  }, [language])

  const loadConversations = useCallback(async () => {
    try {
      const list = await fetchConversations()
      setConversations(list)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load conversations")
    }
  }, [])

  const loadConversation = useCallback(async (id: number) => {
    try {
      const list = await fetchConversations()
      const conv = list.find(c => c.id === id)
      if (conv) {
        setConversation(conv)
        const msgs = await fetchConversationMessages(id)
        setMessages(msgs)
        setError(null)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load conversation")
    }
  }, [])

  const removeConversation = useCallback(async (id: number) => {
    try {
      await deleteConversation(id)
      if (conversation?.id === id) {
        setConversation(null)
        setMessages([])
      }
      setConversations(prev => prev.filter(c => c.id !== id))
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete conversation")
    }
  }, [conversation])

  const abortStream = useCallback(() => {
    if (abortRef.current) {
      abortRef.current.abort()
      abortRef.current = null
      setIsStreaming(false)
    }
  }, [])

  const sendMessage = useCallback((content: string) => {
    if (!conversation || isStreaming) return

    setError(null)
    setIsStreaming(true)

    // Add user message optimistically
    const userMsg: ChatMessage = {
      id: Date.now(), // temporary ID
      role: "user",
      content,
      createdAt: new Date().toISOString(),
    }
    setMessages(prev => [...prev, userMsg])

    // Track streaming assistant message
    let assistantContent = ""
    const toolCallStates: Array<{ name: string; status: string; result: string }> = []

    const abortController = new AbortController()
    abortRef.current = abortController

    sendMessageStream(
      conversation.id,
      content,
      (event: SSEEvent) => {
        switch (event.type) {
          case "tool_call":
            toolCallStates.push({ name: event.name, status: event.status, result: "" })
            // Update messages with tool call indicator
            setMessages(prev => {
              const updated = [...prev]
              const last = updated[updated.length - 1]
              if (last?.role === "assistant" && last.id < 0) {
                // Update existing streaming assistant message
                updated[updated.length - 1] = {
                  ...last,
                  toolCalls: toolCallStates.map(tc => ({
                    name: tc.name,
                    arguments: "",
                    result: tc.result,
                  })),
                }
              } else {
                // Create new streaming assistant message with tool call
                updated.push({
                  id: -1,
                  role: "assistant",
                  content: "",
                  toolCalls: toolCallStates.map(tc => ({
                    name: tc.name,
                    arguments: "",
                    result: tc.result,
                  })),
                  createdAt: new Date().toISOString(),
                })
              }
              return updated
            })
            break

          case "tool_result": {
            // Update tool call result
            const tc = toolCallStates.find(t => t.name === event.name && t.status !== "completed")
            if (tc) {
              tc.status = "completed"
              tc.result = event.result
            }
            setMessages(prev => {
              const updated = [...prev]
              const last = updated[updated.length - 1]
              if (last?.role === "assistant") {
                updated[updated.length - 1] = {
                  ...last,
                  toolCalls: toolCallStates.map(t => ({
                    name: t.name,
                    arguments: "",
                    result: t.result,
                  })),
                }
              }
              return updated
            })
            break
          }

          case "message":
            assistantContent = event.content
            setMessages(prev => {
              const updated = [...prev]
              const last = updated[updated.length - 1]
              if (last?.role === "assistant" && last.id < 0) {
                updated[updated.length - 1] = {
                  ...last,
                  content: assistantContent,
                }
              } else {
                updated.push({
                  id: -1,
                  role: "assistant",
                  content: assistantContent,
                  toolCalls: toolCallStates.length > 0
                    ? toolCallStates.map(tc => ({ name: tc.name, arguments: "", result: tc.result }))
                    : undefined,
                  createdAt: new Date().toISOString(),
                })
              }
              return updated
            })
            break

          case "error":
            setError(event.error)
            setIsStreaming(false)
            break

          case "done":
            setIsStreaming(false)
            // Reload messages from server to get proper IDs
            fetchConversationMessages(conversation.id).then(msgs => {
              setMessages(msgs)
            }).catch(() => {
              // Keep current messages if reload fails
            })
            break
        }
      },
      abortController.signal,
    )
  }, [conversation, isStreaming])

  return {
    conversation,
    conversations,
    messages,
    isStreaming,
    error,
    createNewConversation,
    loadConversation,
    loadConversations,
    removeConversation,
    sendMessage,
    abortStream,
  }
}
