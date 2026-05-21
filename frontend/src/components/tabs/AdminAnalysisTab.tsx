import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Checkbox } from "@/components/ui/checkbox"
import { fetchOCRStatus, startOcrClassification, startOcrClassificationChanges, stopOcrClassification, fetchOcrClassificationStatus, fetchLlmSettings, updateLlmSettings, fetchLlmModels, fetchTagScanStatus, pauseTagScan, resumeTagScan, updateSettings, fetchSettings } from "@/api/endpoints"
import { Shield, Loader2, Zap, Wand2, Play, Square, RefreshCw } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { OCRStatus, OcrClassificationStatusResponse, LlmSettingsDTO, LlmModelDTO, TagScanStatusResponse } from "@/types"

export function AdminAnalysisTab() {
  const { t } = useTranslation()

  const [ocrStatus, setOcrStatus] = useState<OCRStatus | null>(null)
  const [isOcrLoading, setIsOcrLoading] = useState(false)
  const [ocrScanning, setOcrScanning] = useState(false)
  const [ocrScanStatus, setOcrScanStatus] = useState<OcrClassificationStatusResponse | null>(null)
  const [ocrConcurrentWorkers, setOcrConcurrentWorkers] = useState(4)
  const [isSavingWorkers, setIsSavingWorkers] = useState(false)

  // LLM Settings state
  const [llmSettings, setLlmSettings] = useState<LlmSettingsDTO>({
    id: 0,
    provider: "ollama",
    apiUrl: "http://localhost:11434",
    apiKey: "",
    model: "minicpm-v",
    enabled: false,
    tagScanEnabled: false,
    tagScanStartHour: 23,
    tagScanStartMinute: 0,
    tagScanEndHour: 7,
    tagScanEndMinute: 0,
  })
  const [isLlmLoading, setIsLlmLoading] = useState(false)
  const [isLlmSaving, setIsLlmSaving] = useState(false)
  const [llmFormDirty, setLlmFormDirty] = useState(false)
  const [availableModels, setAvailableModels] = useState<LlmModelDTO[]>([])
  const [isModelsLoading, setIsModelsLoading] = useState(false)
  const [showModelInput, setShowModelInput] = useState(false)

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
      // Update tag scan state from LLM settings
      setTagScanEnabled(settings.tagScanEnabled ?? false)
      setTagScanStartHour(settings.tagScanStartHour ?? 23)
      setTagScanStartMinute(settings.tagScanStartMinute ?? 0)
      setTagScanEndHour(settings.tagScanEndHour ?? 7)
      setTagScanEndMinute(settings.tagScanEndMinute ?? 0)
    } catch {
      setLlmSettings({
        id: 0,
        provider: "ollama",
        apiUrl: "http://localhost:11434",
        apiKey: "",
        model: "minicpm-v",
        enabled: false,
        tagScanEnabled: false,
        tagScanStartHour: 23,
        tagScanStartMinute: 0,
        tagScanEndHour: 7,
        tagScanEndMinute: 0,
      })
    } finally {
      setIsLlmLoading(false)
    }
  }, [])

  const handleSaveLlmSettings = useCallback(async () => {
    setIsLlmSaving(true)
    try {
      await updateLlmSettings({
        provider: llmSettings.provider,
        apiUrl: llmSettings.apiUrl,
        apiKey: llmSettings.apiKey,
        model: llmSettings.model,
        enabled: llmSettings.enabled,
      })
      toast.success(t("llm_ocr.settingsSaved"))
      setLlmFormDirty(false)
      await loadLlmSettings()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("llm_ocr.settingsSaveFailed"))
    } finally {
      setIsLlmSaving(false)
    }
  }, [llmSettings, loadLlmSettings, t])

  const handleLlmFieldChange = useCallback((field: keyof LlmSettingsDTO, value: string | boolean) => {
    setLlmSettings((prev) => ({ ...prev, [field]: value }))
    setLlmFormDirty(true)
  }, [])

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
    setIsModelsLoading(true)
    try {
      const response = await fetchLlmModels()
      if (response.success && response.models.length > 0) {
        setAvailableModels(response.models)
        toast.success(`Загружено ${response.models.length} моделей`)
      } else {
        toast.error(response.error || "Не удалось загрузить список моделей")
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Не удалось загрузить список моделей")
    } finally {
      setIsModelsLoading(false)
    }
  }, [])

  // Check OCR classification status on mount to detect already running processes
  const checkInitialOCRStatus = useCallback(async () => {
    try {
      const status = await fetchOcrClassificationStatus()
      if (status.processing) {
        setOcrScanning(true)
        setOcrScanStatus(status)
      }
    } catch {
      // Ignore errors on initial check
    }
  }, [])

  useEffect(() => {
    loadOCRStatus()
    loadLlmSettings()
    checkInitialOCRStatus()
  }, [loadOCRStatus, loadLlmSettings, checkInitialOCRStatus])

  // Load app settings to sync ocrConcurrentWorkers
  useEffect(() => {
    fetchSettings().then((settings) => {
      setOcrConcurrentWorkers(settings.ocrConcurrentRequests ?? 4)
    }).catch(() => {
      // Use local state values
    })
  }, [])

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
              {/* Provider Selection */}
              <div className="space-y-2">
                <Label htmlFor="llm-provider">{t("llm_ocr.provider")}</Label>
                <Select
                  value={llmSettings.provider}
                  onValueChange={(value) => handleLlmFieldChange("provider", value)}
                >
                  <SelectTrigger id="llm-provider">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="ollama">Ollama</SelectItem>
                    <SelectItem value="ollama_cloud">Ollama Cloud</SelectItem>
                    <SelectItem value="openai">OpenAI API</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              {/* API URL */}
              <div className="space-y-2">
                <Label htmlFor="llm-apiurl">API URL</Label>
                <Input
                  id="llm-apiurl"
                  placeholder={
                    llmSettings.provider === "ollama"
                      ? "http://localhost:11434"
                      : llmSettings.provider === "ollama_cloud"
                        ? "https://ollama.com/api"
                        : "https://api.openai.com"
                  }
                  value={llmSettings.apiUrl}
                  onChange={(e) => handleLlmFieldChange("apiUrl", e.target.value)}
                />
              </div>

              {/* API Key (only for OpenAI and Ollama Cloud) */}
              {(llmSettings.provider === "openai" || llmSettings.provider === "ollama_cloud") && (
                <div className="space-y-2">
                  <Label htmlFor="llm-apikey">API Key</Label>
                  <Input
                    id="llm-apikey"
                    type="password"
                    placeholder="sk-..."
                    value={llmSettings.apiKey}
                    onChange={(e) => handleLlmFieldChange("apiKey", e.target.value)}
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
                    Загрузить модели
                  </Button>
                </div>

                {/* Model dropdown */}
                {availableModels.length > 0 && !showModelInput ? (
                  <div className="space-y-2">
                    <Select
                      value={llmSettings.model}
                      onValueChange={(value) => handleLlmFieldChange("model", value)}
                    >
                      <SelectTrigger id="llm-model">
                        <SelectValue placeholder="Выберите модель" />
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
                      Ввести название модели вручную
                    </Button>
                  </div>
                ) : (
                  <div className="space-y-2">
                    <Input
                      id="llm-model"
                      placeholder={
                        llmSettings.provider === "ollama"
                          ? "minicpm-v"
                          : llmSettings.provider === "ollama_cloud"
                            ? "minicpm-v"
                            : "gpt-4-vision-preview"
                      }
                      value={llmSettings.model}
                      onChange={(e) => handleLlmFieldChange("model", e.target.value)}
                    />
                    {availableModels.length > 0 && showModelInput && (
                      <Button
                        variant="link"
                        size="sm"
                        className="px-0 h-auto text-xs"
                        onClick={() => setShowModelInput(false)}
                      >
                        Выбрать из списка доступных моделей
                      </Button>
                    )}
                  </div>
                )}
              </div>

              {/* Enable/Disable Checkbox */}
              <div className="flex items-center space-x-2 rounded-lg border p-3">
                <Checkbox
                  id="llm-enabled"
                  checked={llmSettings.enabled}
                  onCheckedChange={(checked) => handleLlmFieldChange("enabled", checked === true)}
                />
                <div className="space-y-0.5">
                  <Label htmlFor="llm-enabled">{t("llm_ocr.enableRecognition")}</Label>
                  <p className="text-xs text-muted-foreground">
                    {t("llm_ocr.enableDescription")}
                  </p>
                </div>
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
