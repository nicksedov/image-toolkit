import { useCallback, useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Checkbox } from "@/components/ui/checkbox"
import { fetchOCRStatus, startOcrClassification, startOcrClassificationChanges, stopOcrClassification, fetchOcrClassificationStatus, fetchLlmSettings, updateLlmSettings, createLlmProvider, updateLlmProvider, deleteLlmProvider, fetchLlmModels, fetchTagScanStatus, pauseTagScan, resumeTagScan, updateSettings, fetchSettings } from "@/api/endpoints"
import { Shield, Loader2, Zap, Wand2, Play, Square, RefreshCw, Plus, Trash2 } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { OCRStatus, OcrClassificationStatusResponse, LlmSettingsResponse, LlmProviderDTO, LlmModelDTO, TagScanStatusResponse, LlmProviderType } from "@/types"
// Provider type display labels
const PROVIDER_LABELS: Record<LlmProviderType, string> = {
  ollama: "Ollama",
  ollama_cloud: "Ollama Cloud",
  openai: "OpenAI API compatible",
}

// Allowed provider types for new providers
const ALLOWED_PROVIDER_TYPES: LlmProviderType[] = ["ollama", "ollama_cloud", "openai"]

const EMPTY_SETTINGS: LlmSettingsResponse = {
  id: 0,
  activeProvider: "",
  tagScanEnabled: false,
  tagScanStartHour: 23,
  tagScanStartMinute: 0,
  tagScanEndHour: 7,
  tagScanEndMinute: 0,
  providers: [],
}

export function AdminAnalysisTab() {
  const { t } = useTranslation()

  const [ocrStatus, setOcrStatus] = useState<OCRStatus | null>(null)
  const [isOcrLoading, setIsOcrLoading] = useState(false)
  const [ocrScanning, setOcrScanning] = useState(false)
  const [ocrScanStatus, setOcrScanStatus] = useState<OcrClassificationStatusResponse | null>(null)
  const [ocrConcurrentWorkers, setOcrConcurrentWorkers] = useState(4)
  const [isSavingWorkers, setIsSavingWorkers] = useState(false)

  // LLM Settings state
  const [llmSettings, setLlmSettings] = useState<LlmSettingsResponse>(EMPTY_SETTINGS)
  const [isLlmLoading, setIsLlmLoading] = useState(false)
  const [isLlmSaving, setIsLlmSaving] = useState(false)
  const [llmFormDirty, setLlmFormDirty] = useState(false)
  const [availableModels, setAvailableModels] = useState<LlmModelDTO[]>([])
  const [isModelsLoading, setIsModelsLoading] = useState(false)
  const [showModelInput, setShowModelInput] = useState(false)

  // Frontend mirror of DB-backed model cache for instant provider switching
  const modelCacheRef = useRef<Record<string, LlmModelDTO[]>>({})

  // New provider form state
  const [showNewProvider, setShowNewProvider] = useState(false)
  const [newProviderType, setNewProviderType] = useState<LlmProviderType>("ollama")
  const [newProviderAlias, setNewProviderAlias] = useState("")
  const [newProviderApiUrl, setNewProviderApiUrl] = useState("")
  const [newProviderApiKey, setNewProviderApiKey] = useState("")
  const [newProviderModel, setNewProviderModel] = useState("minicpm-v")
 
  // Alias editing state — separate from provider data to avoid collapsing the form on keystroke
  const [editingAlias, setEditingAlias] = useState("")
 
  // Helper to get current active provider
  const getCurrentProvider = useCallback(
    (): LlmProviderDTO | undefined => {
      return llmSettings.providers.find((p) => p.alias === llmSettings.activeProvider)
    },
    [llmSettings.providers, llmSettings.activeProvider]
  )

  // Load models for a provider: uses frontend cache if available, otherwise fetches from backend (which uses DB cache).
  // Pass forceRefresh=true to bypass both caches and re-fetch from the LLM provider.
  const loadModelsForProvider = useCallback(
    async (providerAlias: string, forceRefresh = false) => {
      if (!providerAlias) return

      // Check frontend cache first (mirrors DB cache, populated from settings response or prior fetches)
      if (!forceRefresh && modelCacheRef.current[providerAlias]) {
        setAvailableModels(modelCacheRef.current[providerAlias])
        setShowModelInput(false)
        return
      }

      setIsModelsLoading(true)
      try {
        const response = await fetchLlmModels(providerAlias, forceRefresh)
        if (response.success && response.models.length > 0) {
          modelCacheRef.current[providerAlias] = response.models
          setAvailableModels(response.models)
          setShowModelInput(false)
          toast.success(t("llm_providers.modelsLoaded", { count: response.models.length }))
        } else {
          toast.error(response.error || t("llm_providers.modelsLoadFailed"))
        }
      } catch (err) {
        toast.error(err instanceof Error ? err.message : t("llm_providers.modelsLoadFailed"))
      } finally {
        setIsModelsLoading(false)
      }
    },
    [t]
  )

  // Tag Scan state
  const [tagScanEnabled, setTagScanEnabled] = useState(false)
  const [tagScanStartHour, setTagScanStartHour] = useState(23)
  const [tagScanStartMinute, setTagScanStartMinute] = useState(0)
  const [tagScanEndHour, setTagScanEndHour] = useState(7)
  const [tagScanEndMinute, setTagScanEndMinute] = useState(0)
  const [tagScanStatus, setTagScanStatus] = useState<TagScanStatusResponse | null>(null)
  const [isTagScanSaving, setIsTagScanSaving] = useState(false)
  const [isTagScanPausing, setIsTagScanPausing] = useState(false)
  const [tagScanFormDirty, setTagScanFormDirty] = useState(false)

  const loadOCRStatus = useCallback(async () => {
    try {
      setIsOcrLoading(true)
      const response = await fetchOCRStatus()
      setOcrStatus(response.status)
    } catch {
      setOcrStatus(null)
    } finally {
      setIsOcrLoading(false)
    }
  }, [])

  // Poll OCR scan status when scanning
  useEffect(() => {
    if (!ocrScanning) return

    let cancelled = false

    const checkStatus = async () => {
      try {
        const status = await fetchOcrClassificationStatus()
        if (cancelled) return
        setOcrScanStatus(status)
        setOcrScanning(status.processing)
      } catch (err) {
        console.error("Failed to check OCR scan status:", err)
      }
    }

    checkStatus()
    const interval = setInterval(() => {
      if (!cancelled) {
        checkStatus()
      }
    }, 2000)

    return () => {
      cancelled = true
      clearInterval(interval)
    }
  }, [ocrScanning])

  const handleStartOcrScanAll = useCallback(async () => {
    try {
      await startOcrClassification()
      setOcrScanning(true)
      toast.success(t("api.ocr.started"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("api.ocr.failed"))
    }
  }, [t])

  const handleStartOcrScanChanges = useCallback(async () => {
    try {
      await startOcrClassificationChanges()
      setOcrScanning(true)
      toast.success(t("api.ocr.started"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("api.ocr.failed"))
    }
  }, [t])

  const handleStopOcrScan = useCallback(async () => {
    try {
      await stopOcrClassification()
      toast.info("OCR scanning stopping...")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("api.ocr.failed"))
    }
  }, [t])

  const handleSaveOcrWorkers = useCallback(async () => {
    setIsSavingWorkers(true)
    try {
      await updateSettings({ ocrConcurrentRequests: ocrConcurrentWorkers })
      toast.success(t("adminPanel.ocr.concurrentWorkersSaved"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("adminPanel.ocr.concurrentWorkersSaveFailed"))
    } finally {
      setIsSavingWorkers(false)
    }
  }, [ocrConcurrentWorkers, t])

  const handleWorkersInputChange = useCallback((value: string) => {
    const num = parseInt(value, 10)
    if (!isNaN(num) && num >= 0) {
      setOcrConcurrentWorkers(num)
    } else if (value === "") {
      setOcrConcurrentWorkers(0)
    }
  }, [])

  const loadLlmSettings = useCallback(async () => {
    try {
      setIsLlmLoading(true)
      const settings = await fetchLlmSettings()
      setLlmSettings(settings)
      setLlmFormDirty(false)
      setShowNewProvider(false)
      setNewProviderAlias("")
      setNewProviderApiUrl("")
      // Sync alias editing state with current provider
      const active = settings.providers.find((p) => p.alias === settings.activeProvider)
      setEditingAlias(active?.alias ?? "")
      // Update tag scan state from LLM settings
      setTagScanEnabled(settings.tagScanEnabled ?? false)
      setTagScanStartHour(settings.tagScanStartHour ?? 23)
      setTagScanStartMinute(settings.tagScanStartMinute ?? 0)
      setTagScanEndHour(settings.tagScanEndHour ?? 7)
      setTagScanEndMinute(settings.tagScanEndMinute ?? 0)
    } catch {
      setLlmSettings(EMPTY_SETTINGS)
    } finally {
      setIsLlmLoading(false)
    }
  }, [])

  const handleSaveLlmSettings = useCallback(async () => {
    setIsLlmSaving(true)
    try {
      const currentProvider = getCurrentProvider()

      // Save active provider and tag scan settings
      await updateLlmSettings({
        activeProvider: llmSettings.activeProvider,
        tagScanEnabled: llmSettings.tagScanEnabled,
        tagScanStartHour: llmSettings.tagScanStartHour,
        tagScanStartMinute: llmSettings.tagScanStartMinute,
        tagScanEndHour: llmSettings.tagScanEndHour,
        tagScanEndMinute: llmSettings.tagScanEndMinute,
        tagScanTimezoneOffset: new Date().getTimezoneOffset(),
      })

      // Save current provider settings if exists — uses dedicated provider endpoint
      if (currentProvider) {
      	const provUpdate: { apiUrl?: string; apiKey?: string; model?: string } = {
      		apiUrl: currentProvider.apiUrl,
      		model: currentProvider.model,
      	}
      	// Only send API key if it was changed by the user (not masked)
      	// Masked key format: "XXXX...XXXX" (4 chars + "..." + 4 chars = 11 chars)
      	const isMasked = /^.{4}\.\.\..{4}$/.test(currentProvider.apiKey) && currentProvider.apiKey.length === 11
      	if (!isMasked) {
      		provUpdate.apiKey = currentProvider.apiKey
      	}
      	await updateLlmProvider(currentProvider.alias, provUpdate)
      }

      toast.success(t("llm_ocr.settingsSaved"))
      setLlmFormDirty(false)
      await loadLlmSettings()
    } catch {
    	toast.error(t("llm_ocr.settingsSaveFailed"))
    } finally {
      setIsLlmSaving(false)
    }
  }, [llmSettings, loadLlmSettings, getCurrentProvider, t])

  // Update a field on a specific provider identified by alias
  const handleProviderFieldChange = useCallback((alias: string, field: keyof LlmProviderDTO, value: string | boolean) => {
    setLlmSettings((prev) => {
      const providers = prev.providers.map((p) => {
        if (p.alias === alias) {
          return { ...p, [field]: value }
        }
        return p
      })
      return { ...prev, providers }
    })
    setLlmFormDirty(true)
  }, [])

  // Update alias for a specific provider (backend + local)
  const handleAliasUpdate = useCallback(async (oldAlias: string, newAlias: string) => {
  	if (!newAlias.trim() || newAlias === oldAlias) return
 
  	// Check uniqueness
  	if (llmSettings.providers.some((p) => p.alias === newAlias && p.alias !== oldAlias)) {
  		toast.error(t("llm_providers.aliasMustBeUnique"))
  		return
  	}
 
  	setIsLlmSaving(true)
  	try {
  		await updateLlmProvider(oldAlias, { alias: newAlias })
  		toast.success(t("llm_ocr.settingsSaved"))
  		// Migrate frontend cache to new alias
  		if (modelCacheRef.current[oldAlias]) {
  			modelCacheRef.current[newAlias] = modelCacheRef.current[oldAlias]
  			delete modelCacheRef.current[oldAlias]
  		}
  		await loadLlmSettings()
  	} catch {
  		toast.error(t("llm_ocr.settingsSaveFailed"))
  	} finally {
  		setIsLlmSaving(false)
  	}
  }, [llmSettings.providers, loadLlmSettings, t])

  // Delete a provider
  const handleDeleteProvider = useCallback(async (alias: string) => {
  	if (!confirm(t("llm_providers.deleteConfirm", { alias }))) return
 
  	setIsLlmSaving(true)
  	try {
  		await deleteLlmProvider(alias)
  		toast.success(t("llm_ocr.settingsSaved"))
  		setLlmFormDirty(false)
  		// Clean up frontend cache for deleted provider
  		delete modelCacheRef.current[alias]
  		await loadLlmSettings()
  	} catch {
  		toast.error(t("llm_ocr.settingsSaveFailed"))
  	} finally {
  		setIsLlmSaving(false)
  	}
  }, [loadLlmSettings, t])

  // Add a new provider
  const handleAddProvider = useCallback(async () => {
  	if (!newProviderAlias.trim()) {
  		toast.error("Alias is required")
  		return
  	}

  	// Check uniqueness
  	if (llmSettings.providers.some((p) => p.alias === newProviderAlias.trim())) {
  		toast.error(t("llm_providers.aliasMustBeUnique"))
  		return
  	}

  	// Resolve API URL: ollama_cloud uses predefined URL, others use user input with defaults
  	const defaultApiUrl = newProviderType === "ollama" ? "http://localhost:11434" : newProviderType === "ollama_cloud" ? "https://ollama.com" : "https://api.openai.com"
  	const apiUrl = newProviderType === "ollama_cloud" ? defaultApiUrl : (newProviderApiUrl.trim() || defaultApiUrl)

  	setIsLlmSaving(true)
  	try {
  		await createLlmProvider({
  			alias: newProviderAlias.trim(),
  			name: newProviderType,
  			apiUrl,
  			apiKey: (newProviderType === "ollama_cloud" || newProviderType === "openai") ? newProviderApiKey : undefined,
  			model: newProviderModel || "minicpm-v",
  		})
  		toast.success(t("llm_ocr.settingsSaved"))
  		setShowNewProvider(false)
  		setNewProviderAlias("")
  		setNewProviderApiUrl("")
  		setNewProviderApiKey("")
  		setNewProviderModel("minicpm-v")
  		setLlmFormDirty(false)
  		await loadLlmSettings()
  	} catch {
  		toast.error(t("llm_ocr.settingsSaveFailed"))
  	} finally {
  		setIsLlmSaving(false)
  	}
  }, [newProviderAlias, newProviderType, newProviderApiUrl, newProviderApiKey, newProviderModel, llmSettings.providers, loadLlmSettings, t])

  const handleActiveProviderChange = useCallback((value: string) => {
    setLlmSettings((prev) => ({ ...prev, activeProvider: value }))
    setLlmFormDirty(true)
    loadModelsForProvider(value)
  }, [loadModelsForProvider])

  // Tag Scan handlers
  const loadTagScanStatus = useCallback(async () => {
    try {
      const status = await fetchTagScanStatus()
      setTagScanStatus(status)
    } catch {
      setTagScanStatus(null)
    }
  }, [])

  const handleSaveTagScanSchedule = useCallback(async () => {
    setIsTagScanSaving(true)
    try {
      await updateLlmSettings({
        tagScanEnabled,
        tagScanStartHour,
        tagScanStartMinute,
        tagScanEndHour,
        tagScanEndMinute,
        tagScanTimezoneOffset: new Date().getTimezoneOffset(),
      })
      toast.success(t("tagScan.saved"))
      setTagScanFormDirty(false)
      await loadTagScanStatus()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("tagScan.saveFailed"))
    } finally {
      setIsTagScanSaving(false)
    }
  }, [tagScanEnabled, tagScanStartHour, tagScanStartMinute, tagScanEndHour, tagScanEndMinute, loadTagScanStatus, t])

  const handleTagScanFieldChange = useCallback((field: string, value: string | boolean | number) => {
    switch (field) {
      case "tagScanEnabled": setTagScanEnabled(value as boolean); break;
      case "tagScanStartHour": setTagScanStartHour(value as number); break;
      case "tagScanStartMinute": setTagScanStartMinute(value as number); break;
      case "tagScanEndHour": setTagScanEndHour(value as number); break;
      case "tagScanEndMinute": setTagScanEndMinute(value as number); break;
    }
    setTagScanFormDirty(true)
  }, [])

  const handlePauseTagScan = useCallback(async () => {
    setIsTagScanPausing(true)
    try {
      await pauseTagScan()
      toast.info(t("tagScan.paused"))
      await loadTagScanStatus()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("tagScan.pauseFailed"))
    } finally {
      setIsTagScanPausing(false)
    }
  }, [loadTagScanStatus, t])

  const handleResumeTagScan = useCallback(async () => {
    setIsTagScanPausing(true)
    try {
      await resumeTagScan()
      toast.info(t("tagScan.resumed"))
      await loadTagScanStatus()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("tagScan.resumeFailed"))
    } finally {
      setIsTagScanPausing(false)
    }
  }, [loadTagScanStatus, t])

  // Poll tag scan status periodically with adaptive interval
  useEffect(() => {
    let cancelled = false
    let timeoutId: ReturnType<typeof setTimeout> | null = null

    const scheduleNext = async () => {
      if (cancelled) return
      try {
        const status = await fetchTagScanStatus()
        if (cancelled) return
        setTagScanStatus(status)

        const isActive = status?.running && !status?.paused
        const nextDelay = isActive ? 10000 : 30000
        timeoutId = setTimeout(scheduleNext, nextDelay)
      } catch {
        if (!cancelled) {
          setTagScanStatus(null)
        }
        timeoutId = setTimeout(scheduleNext, 30000)
      }
    }

    scheduleNext()

    return () => {
      cancelled = true
      if (timeoutId) clearTimeout(timeoutId)
    }
  }, [])

  const handleLoadModels = useCallback(async () => {
    const currentProvider = getCurrentProvider()
    if (!currentProvider) return
    await loadModelsForProvider(currentProvider.alias, true)
  }, [getCurrentProvider, loadModelsForProvider])

  useEffect(() => {
    const init = async () => {
      // Load OCR status
      try {
        setIsOcrLoading(true)
        const response = await fetchOCRStatus()
        setOcrStatus(response.status)
      } catch {
        setOcrStatus(null)
      } finally {
        setIsOcrLoading(false)
      }

      // Load LLM settings
      try {
        setIsLlmLoading(true)
        const settings = await fetchLlmSettings()
        setLlmSettings(settings)
        setLlmFormDirty(false)
        setShowNewProvider(false)
        setNewProviderAlias("")
        setNewProviderApiUrl("")
        const active = settings.providers.find((p) => p.alias === settings.activeProvider)
        setEditingAlias(active?.alias ?? "")
        setTagScanEnabled(settings.tagScanEnabled ?? false)
        setTagScanStartHour(settings.tagScanStartHour ?? 23)
        setTagScanStartMinute(settings.tagScanStartMinute ?? 0)
        setTagScanEndHour(settings.tagScanEndHour ?? 7)
        setTagScanEndMinute(settings.tagScanEndMinute ?? 0)

        // Seed frontend cache from DB-backed cachedModels included in settings response
        for (const p of settings.providers) {
          if (p.cachedModels && p.cachedModels.length > 0) {
            modelCacheRef.current[p.alias] = p.cachedModels
          }
        }
        // Auto-populate models for active provider (instant from cache, or auto-fetch)
        if (settings.activeProvider) {
          loadModelsForProvider(settings.activeProvider)
        }
      } catch {
        setLlmSettings(EMPTY_SETTINGS)
      } finally {
        setIsLlmLoading(false)
      }

      // Check initial OCR classification status
      try {
        const status = await fetchOcrClassificationStatus()
        if (status.processing) {
          setOcrScanning(true)
          setOcrScanStatus(status)
        }
      } catch {
        // Ignore errors on initial check
      }
    }

    init()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loadModelsForProvider])

  // Load app settings to sync ocrConcurrentWorkers
  useEffect(() => {
    fetchSettings().then((settings) => {
      setOcrConcurrentWorkers(settings.ocrConcurrentRequests ?? 4)
    }).catch(() => {
      // Use local state values
    })
  }, [])

  const currentProvider = getCurrentProvider()

  // Keep editingAlias in sync when active provider changes (render-phase update avoids cascading renders)
  if (currentProvider && editingAlias !== currentProvider.alias) {
    setEditingAlias(currentProvider.alias)
  }

  // Provider type display name lookup
  const getProviderLabel = (name: LlmProviderType): string => {
  	return PROVIDER_LABELS[name] ?? name
  }

  return (
    <div className="space-y-6">
      {/* OCR Document Search */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Shield className="h-5 w-5" />
            {t("adminPanel.ocr.title")}
          </CardTitle>
          <CardDescription>{t("adminPanel.ocr.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* OCR Service Status */}
          <div className="flex items-center justify-between rounded-lg border p-3">
            <div className="space-y-1">
              <div className="text-sm font-medium">{t("adminPanel.ocr.status")}</div>
              {isOcrLoading ? (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {t("common.loading")}
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <span
                    className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                      ocrStatus?.enabled && ocrStatus?.health === "healthy"
                        ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
                        : ocrStatus?.error || (ocrStatus?.enabled && ocrStatus?.health !== "healthy")
                        ? "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200"
                        : "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200"
                    }`}
                  >
                    {ocrStatus?.enabled && ocrStatus?.health === "healthy"
                      ? t("adminPanel.ocr.statusHealthy")
                      : ocrStatus?.error || (ocrStatus?.enabled && ocrStatus?.health !== "healthy")
                      ? t("adminPanel.ocr.statusError")
                      : t("adminPanel.ocr.statusDisabled")}
                  </span>
                </div>
              )}
            </div>
            <Button variant="outline" size="sm" onClick={loadOCRStatus} disabled={isOcrLoading}>
              {isOcrLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
            </Button>
          </div>

          {/* Concurrent Workers */}
          <div className="space-y-2">
            <Label htmlFor="ocr-workers-input">{t("adminPanel.ocr.concurrentWorkers")}</Label>
            <p className="text-xs text-muted-foreground">{t("adminPanel.ocr.concurrentWorkersDescription")}</p>
            <div className="flex gap-2">
              <Input
                id="ocr-workers-input"
                type="number"
                min={0}
                value={ocrConcurrentWorkers}
                onChange={(e) => handleWorkersInputChange(e.target.value)}
                className="w-24"
              />
              <Button
                onClick={handleSaveOcrWorkers}
                disabled={isSavingWorkers}
                size="default"
              >
                {isSavingWorkers ? t("common.saving") : t("common.save")}
              </Button>
            </div>
          </div>

          {/* OCR Scan Progress */}
          {ocrScanning && ocrScanStatus && (
            <div className="p-4 bg-muted rounded-lg">
              <div className="flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span className="text-sm">
                  {t("ocr.filesProcessed", {
                    count: ocrScanStatus.filesProcessed,
                    total: ocrScanStatus.totalFiles,
                  })}
                </span>
              </div>
              <p className="text-xs text-muted-foreground mt-1">{ocrScanStatus.progress}</p>
            </div>
          )}

          {/* Scan Buttons */}
          <div className="flex gap-2">
            <Button
              onClick={handleStartOcrScanChanges}
              disabled={ocrScanning}
              variant="outline"
              size="sm"
            >
              <Zap className={`mr-1.5 h-3.5 w-3.5 ${ocrScanning ? "animate-spin" : ""}`} />
              {t("adminPanel.ocr.scanChanges")}
            </Button>
            <Button
              onClick={handleStartOcrScanAll}
              disabled={ocrScanning}
              variant="outline"
              size="sm"
            >
              <Play className={`mr-1.5 h-3.5 w-3.5 ${ocrScanning ? "animate-spin" : ""}`} />
              {t("adminPanel.ocr.scanAll")}
            </Button>
            {ocrScanning && (
              <Button
                onClick={handleStopOcrScan}
                variant="destructive"
                size="sm"
              >
                <Square className="mr-1.5 h-3.5 w-3.5" />
                {t("adminPanel.ocr.stopScanning")}
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      {/* LLM Settings */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Wand2 className="h-5 w-5" />
            {t("llm_ocr.settings")}
          </CardTitle>
          <CardDescription>Configure AI-powered features: image description, tag generation, text recognition, and visual question answering</CardDescription>
        </CardHeader>
        <CardContent>
          {isLlmLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <div className="space-y-4">
              {/* Active Provider Selection */}
              <div className="space-y-2">
                <Label htmlFor="llm-provider">{t("llm_ocr.provider")}</Label>
                <Select
                  value={llmSettings.activeProvider}
                  onValueChange={handleActiveProviderChange}
                >
                  <SelectTrigger id="llm-provider">
                    <SelectValue placeholder={t("llm_providers.selectProvider")} />
                  </SelectTrigger>
                  <SelectContent>
                    {llmSettings.providers.map((p) => (
                      <SelectItem key={p.alias} value={p.alias}>
                        {p.alias} ({getProviderLabel(p.name)})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {/* Current Provider Settings */}
              {currentProvider && (
                <div className="space-y-4 rounded-lg border p-4">
                  <div className="flex items-center justify-between">
                    <h4 className="text-sm font-medium">
                      {t("llm_providers.providerLabel", { alias: currentProvider.alias })}
                      <span className="ml-2 text-xs text-muted-foreground">
                        ({getProviderLabel(currentProvider.name)})
                      </span>
                    </h4>
                    <div className="flex gap-2">
                      {llmSettings.providers.length > 1 && (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleDeleteProvider(currentProvider.alias)}
                          className="h-8 w-8 p-0 text-destructive"
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      )}
                    </div>
                  </div>

                  {/* Alias field */}
                  <div className="space-y-2">
                    <Label htmlFor="llm-alias">{t("llm_providers.alias")}</Label>
                    <div className="flex gap-2">
                      <Input
                        id="llm-alias"
                        value={editingAlias}
                        onChange={(e) => setEditingAlias(e.target.value)}
                      />
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          if (editingAlias !== currentProvider.alias) {
                            handleAliasUpdate(currentProvider.alias, editingAlias)
                          }
                        }}
                        disabled={isLlmSaving || editingAlias === currentProvider.alias || !editingAlias.trim()}
                      >
                        {isLlmSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : t("llm_providers.rename")}
                      </Button>
                    </div>
                  </div>

                  {/* API URL (hidden for ollama_cloud — predefined) */}
                  {currentProvider.name !== "ollama_cloud" && (
                    <div className="space-y-2">
                      <Label htmlFor="llm-apiurl">API URL</Label>
                      <Input
                        id="llm-apiurl"
                        placeholder={currentProvider.name === "ollama" ? "http://localhost:11434" : "https://api.openai.com"}
                        value={currentProvider.apiUrl}
                        onChange={(e) => handleProviderFieldChange(currentProvider.alias, "apiUrl", e.target.value)}
                      />
                    </div>
                  )}

                  {/* API Key (only for OpenAI and Ollama Cloud) */}
                  {(currentProvider.name === "openai" || currentProvider.name === "ollama_cloud") && (
                  	<div className="space-y-2">
                  	  <Label htmlFor="llm-apikey">API Key</Label>
                  	  <Input
                  	    id="llm-apikey"
                  	    type="password"
                  	    autoComplete="new-password"
                  	    placeholder="sk-..."
                  	    value={currentProvider.apiKey}
                  	    onChange={(e) => handleProviderFieldChange(currentProvider.alias, "apiKey", e.target.value)}
                  	  />
                  	</div>
                  )}

                  {/* Model */}
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <Label htmlFor="llm-model">{t("llm_ocr.model")}</Label>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={handleLoadModels}
                        disabled={isModelsLoading}
                        className="h-6 px-2 text-xs"
                      >
                        {isModelsLoading ? (
                          <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                        ) : (
                          <RefreshCw className="mr-1 h-3 w-3" />
                        )}
                        {t("llm_providers.loadModels")}
                      </Button>
                    </div>

                    {/* Model dropdown or input */}
                    {availableModels.length > 0 && !showModelInput ? (
                      <div className="space-y-2">
                        <Select
                          value={currentProvider.model}
                          onValueChange={(value) => handleProviderFieldChange(currentProvider.alias, "model", value)}
                        >
                          <SelectTrigger id="llm-model">
                            <SelectValue placeholder={t("llm_providers.selectModel")} />
                          </SelectTrigger>
                          <SelectContent>
                            {availableModels.map((model) => (
                              <SelectItem key={model.id} value={model.id}>
                                {model.name}
                                {model.size ? ` (${(model.size / 1073741824).toFixed(1)} GB)` : ""}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <Button
                          variant="link"
                          size="sm"
                          className="px-0 h-auto text-xs"
                          onClick={() => setShowModelInput(true)}
                        >
                          {t("llm_providers.enterModelManually")}
                        </Button>
                      </div>
                    ) : (
                      <div className="space-y-2">
                        <Input
                          id="llm-model"
                          placeholder={
                            currentProvider.name === "ollama" || currentProvider.name === "ollama_cloud"
                              ? "minicpm-v"
                              : "gpt-4-vision-preview"
                          }
                          value={currentProvider.model}
                          onChange={(e) => handleProviderFieldChange(currentProvider.alias, "model", e.target.value)}
                        />
                        {availableModels.length > 0 && showModelInput && (
                          <Button
                            variant="link"
                            size="sm"
                            className="px-0 h-auto text-xs"
                            onClick={() => setShowModelInput(false)}
                          >
                            {t("llm_providers.selectFromModels")}
                          </Button>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* No providers message */}
              {llmSettings.providers.length === 0 && (
                <p className="text-sm text-muted-foreground text-center py-4">
                  {t("llm_providers.noProviders")}
                </p>
              )}

              {/* Add New Provider */}
              <div className="border-t pt-4">
                {showNewProvider ? (
                  <div className="space-y-3 rounded-lg border p-4">
                    <h4 className="text-sm font-medium">{t("llm_providers.newProvider")}</h4>

                    {/* Provider Type */}
                    <div className="space-y-2">
                      <Label>{t("llm_providers.type")}</Label>
                      <Select
                        value={newProviderType}
                        onValueChange={(v) => setNewProviderType(v as LlmProviderType)}
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {ALLOWED_PROVIDER_TYPES.map((type) => (
                            <SelectItem key={type} value={type}>
                              {getProviderLabel(type)}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    {/* Alias */}
                    <div className="space-y-2">
                      <Label htmlFor="new-alias">{t("llm_providers.alias")}</Label>
                      <Input
                        id="new-alias"
                        placeholder={t("llm_providers.aliasPlaceholder")}
                        value={newProviderAlias}
                        onChange={(e) => setNewProviderAlias(e.target.value)}
                      />
                    </div>

                    {/* API URL (hidden for ollama_cloud — predefined) */}
                    {newProviderType !== "ollama_cloud" && (
                      <div className="space-y-2">
                        <Label htmlFor="new-apiurl">API URL</Label>
                        <Input
                          id="new-apiurl"
                          placeholder={newProviderType === "ollama" ? "http://localhost:11434" : "https://api.openai.com"}
                          value={newProviderApiUrl}
                          onChange={(e) => setNewProviderApiUrl(e.target.value)}
                        />
                      </div>
                    )}

                    {/* API Key (only for Ollama Cloud and OpenAI) */}
                    {(newProviderType === "ollama_cloud" || newProviderType === "openai") && (
                      <div className="space-y-2">
                        <Label htmlFor="new-apikey">API Key</Label>
                        <Input
                          id="new-apikey"
                          type="password"
                          autoComplete="new-password"
                          placeholder="sk-..."
                          value={newProviderApiKey}
                          onChange={(e) => setNewProviderApiKey(e.target.value)}
                        />
                      </div>
                    )}

                    {/* Model */}
                    <div className="space-y-2">
                      <div className="flex items-center justify-between">
                        <Label htmlFor="new-model">{t("llm_ocr.model")}</Label>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={handleLoadModels}
                          disabled={isModelsLoading || !llmSettings.activeProvider}
                          className="h-6 px-2 text-xs"
                        >
                          {isModelsLoading ? (
                            <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                          ) : (
                            <RefreshCw className="mr-1 h-3 w-3" />
                          )}
                          {t("llm_providers.loadModels")}
                        </Button>
                      </div>

                      {availableModels.length > 0 && !showModelInput ? (
                        <div className="space-y-2">
                          <Select
                            value={newProviderModel}
                            onValueChange={(value) => setNewProviderModel(value)}
                          >
                            <SelectTrigger id="new-model">
                              <SelectValue placeholder={t("llm_providers.selectModel")} />
                            </SelectTrigger>
                            <SelectContent>
                              {availableModels.map((model) => (
                                <SelectItem key={model.id} value={model.id}>
                                  {model.name}
                                  {model.size ? ` (${(model.size / 1073741824).toFixed(1)} GB)` : ""}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                          <Button
                            variant="link"
                            size="sm"
                            className="px-0 h-auto text-xs"
                            onClick={() => setShowModelInput(true)}
                          >
                            {t("llm_providers.enterModelManually")}
                          </Button>
                        </div>
                      ) : (
                        <div className="space-y-2">
                          <Input
                            id="new-model"
                            placeholder="minicpm-v"
                            value={newProviderModel}
                            onChange={(e) => setNewProviderModel(e.target.value)}
                          />
                          {availableModels.length > 0 && showModelInput && (
                            <Button
                              variant="link"
                              size="sm"
                              className="px-0 h-auto text-xs"
                              onClick={() => setShowModelInput(false)}
                            >
                              {t("llm_providers.selectFromModels")}
                            </Button>
                          )}
                        </div>
                      )}
                    </div>

                    <div className="flex gap-2 justify-end">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          setShowNewProvider(false)
                          setNewProviderAlias("")
                          setNewProviderApiUrl("")
                          setNewProviderApiKey("")
                        }}
                      >
                        {t("common.cancel")}
                      </Button>
                      <Button
                        size="sm"
                        onClick={handleAddProvider}
                        disabled={isLlmSaving || !newProviderAlias.trim()}
                      >
                        {isLlmSaving ? <Loader2 className="mr-1 h-4 w-4 animate-spin" /> : <Plus className="mr-1 h-4 w-4" />}
                        {t("llm_providers.add")}
                      </Button>
                    </div>
                  </div>
                ) : (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setShowNewProvider(true)}
                    className="w-full"
                  >
                    <Plus className="mr-1.5 h-4 w-4" />
                    {t("llm_providers.addProvider")}
                  </Button>
                )}
              </div>

              {/* Save Button */}
              <div className="flex justify-end pt-2">
                <Button
                  onClick={handleSaveLlmSettings}
                  disabled={isLlmSaving || !llmFormDirty}
                >
                  {isLlmSaving ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      {t("common.saving")}
                    </>
                  ) : (
                    t("common.save")
                  )}
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Tag Scan Settings */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Wand2 className="h-5 w-5" />
            {t("tagScan.title")}
          </CardTitle>
          <CardDescription>{t("tagScan.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Enable/Disable Checkbox */}
          <div className="flex items-center space-x-2 rounded-lg border p-3">
            <Checkbox
              id="tag-scan-enabled"
              checked={tagScanEnabled}
              onCheckedChange={(checked) => handleTagScanFieldChange("tagScanEnabled", checked === true)}
            />
            <div className="space-y-0.5">
              <Label htmlFor="tag-scan-enabled">{t("tagScan.enabled")}</Label>
              <p className="text-xs text-muted-foreground">
                {t("tagScan.description")}
              </p>
            </div>
          </div>

          {tagScanEnabled && (
            <>
              {/* Schedule */}
              <div className="space-y-2">
                <Label>{t("tagScan.schedule")}</Label>
                <div className="flex items-center gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="tag-scan-start-hour">{t("tagScan.startTime")}</Label>
                    <div className="flex gap-2">
                      <Select
                        value={String(tagScanStartHour)}
                        onValueChange={(val) => handleTagScanFieldChange("tagScanStartHour", Number(val))}
                      >
                        <SelectTrigger id="tag-scan-start-hour" className="w-20">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {Array.from({ length: 24 }, (_, i) => i).map((h) => (
                            <SelectItem key={h} value={String(h)}>
                              {String(h).padStart(2, "0")}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <span className="self-center text-muted-foreground">:</span>
                      <Select
                        value={String(tagScanStartMinute)}
                        onValueChange={(val) => handleTagScanFieldChange("tagScanStartMinute", Number(val))}
                      >
                        <SelectTrigger id="tag-scan-start-minute" className="w-20">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {Array.from({ length: 12 }, (_, i) => i * 5).map((m) => (
                            <SelectItem key={m} value={String(m)}>
                              {String(m).padStart(2, "0")}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="tag-scan-end-hour">{t("tagScan.endTime")}</Label>
                    <div className="flex gap-2">
                      <Select
                        value={String(tagScanEndHour)}
                        onValueChange={(val) => handleTagScanFieldChange("tagScanEndHour", Number(val))}
                      >
                        <SelectTrigger id="tag-scan-end-hour" className="w-20">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {Array.from({ length: 24 }, (_, i) => i).map((h) => (
                            <SelectItem key={h} value={String(h)}>
                              {String(h).padStart(2, "0")}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <span className="self-center text-muted-foreground">:</span>
                      <Select
                        value={String(tagScanEndMinute)}
                        onValueChange={(val) => handleTagScanFieldChange("tagScanEndMinute", Number(val))}
                      >
                        <SelectTrigger id="tag-scan-end-minute" className="w-20">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {Array.from({ length: 12 }, (_, i) => i * 5).map((m) => (
                            <SelectItem key={m} value={String(m)}>
                              {String(m).padStart(2, "0")}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                </div>
              </div>

              {/* Status and Progress */}
              {tagScanStatus && (
                <div className="space-y-2">
                  <Label>{t("tagScan.status")}</Label>
                  <div className="flex items-center gap-4 rounded-lg border p-3">
                    <div className="flex items-center gap-2">
                      <span
                        className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                          tagScanStatus.running && !tagScanStatus.paused
                            ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
                            : tagScanStatus.paused
                            ? "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200"
                            : "bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200"
                        }`}
                      >
                        {tagScanStatus.running && !tagScanStatus.paused
                          ? t("tagScan.running")
                          : tagScanStatus.paused
                          ? t("tagScan.paused")
                          : t("tagScan.stopped")}
                      </span>
                    </div>

                    <div className="flex-1 text-sm text-muted-foreground">
                      {tagScanStatus.scanned} {t("tagScan.of")} {tagScanStatus.total} {t("tagScan.images")}
                    </div>

                    <div className="flex gap-2">
                      {tagScanStatus.running && !tagScanStatus.paused ? (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={handlePauseTagScan}
                          disabled={isTagScanPausing}
                        >
                          <Square className="mr-1.5 h-3.5 w-3.5" />
                          {isTagScanPausing ? t("common.saving") : t("tagScan.pause")}
                        </Button>
                      ) : (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={handleResumeTagScan}
                          disabled={isTagScanPausing || !tagScanStatus.running}
                        >
                          <Play className="mr-1.5 h-3.5 w-3.5" />
                          {isTagScanPausing ? t("common.saving") : t("tagScan.resume")}
                        </Button>
                      )}
                    </div>
                  </div>

                  {/* Progress Bar */}
                  {tagScanStatus.total > 0 && (
                    <div className="w-full h-2 bg-muted rounded-full overflow-hidden">
                      <div
                        className="h-full bg-primary transition-all duration-300"
                        style={{ width: `${(tagScanStatus.scanned / tagScanStatus.total) * 100}%` }}
                      />
                    </div>
                  )}

                  {/* Current Image */}
                  {tagScanStatus.currentImage && (
                    <p className="text-xs text-muted-foreground">
                      {t("tagScan.currentImage")}: {tagScanStatus.currentImage}
                    </p>
                  )}

                  {/* Last Error */}
                  {tagScanStatus.lastError && (
                    <p className="text-xs text-destructive">
                      {t("tagScan.lastError")}: {tagScanStatus.lastError}
                    </p>
                  )}
                </div>
              )}

              {/* Save Button */}
              <div className="flex justify-end pt-2">
                <Button
                  onClick={handleSaveTagScanSchedule}
                  disabled={isTagScanSaving || !tagScanFormDirty}
                >
                  {isTagScanSaving ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      {t("common.saving")}
                    </>
                  ) : (
                    t("tagScan.save")
                  )}
                </Button>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
