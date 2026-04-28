import { useEffect, useRef, useState, useCallback } from "react"
import { Button } from "@/components/ui/button"
import { FileText, Play, Loader2 } from "lucide-react"
import { useTranslation } from "@/i18n"
import { startOcrClassification, fetchOcrClassificationStatus } from "@/api/endpoints"
import type { OcrDocumentDTO, OcrClassificationStatusResponse } from "@/types"
import { OcrLightbox } from "@/components/gallery/OcrLightbox"
import { toast } from "sonner"
import { useOcrDocuments } from "@/hooks/useOcrDocuments"

export function OcrTab() {
  const { t } = useTranslation()
  const { documents, totalDocuments, hasMore, isLoading, loadMore, reset } = useOcrDocuments()
  const [scanning, setScanning] = useState(false)
  const [scanStatus, setScanStatus] = useState<OcrClassificationStatusResponse | null>(null)
  const [selectedImage, setSelectedImage] = useState<string | null>(null)

  // Sentinel ref for infinite scroll
  const sentinelRef = useRef<HTMLDivElement>(null)
  const observerRef = useRef<IntersectionObserver | null>(null)

  // Poll scan status when scanning
  useEffect(() => {
    if (!scanning) return

    let cancelled = false

    const checkStatus = async () => {
      try {
        const status = await fetchOcrClassificationStatus()
        if (cancelled) return
        setScanStatus(status)
        setScanning(status.processing)
        if (!status.processing) {
          // Scan just finished, reload documents
          reset()
          loadMore()
        }
      } catch (err) {
        console.error("Failed to check scan status:", err)
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
  }, [scanning, reset, loadMore])

  // Load initial documents on mount
  useEffect(() => {
    loadMore()
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // Set up intersection observer for infinite scroll
  useEffect(() => {
    const sentinel = sentinelRef.current
    if (!sentinel) return

    observerRef.current = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasMore && !isLoading) {
          loadMore()
        }
      },
      {
        root: null,
        rootMargin: "400px", // Prefetch 400px before reaching bottom
        threshold: 0.1,
      }
    )

    observerRef.current.observe(sentinel)

    return () => {
      if (observerRef.current) {
        observerRef.current.disconnect()
      }
    }
  }, [hasMore, isLoading, loadMore])

  const handleStartScan = async () => {
    try {
      await startOcrClassification()
      setScanning(true)
      toast.success(t("api.ocr.started"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("api.ocr.failed"))
    }
  }

  const handleDocumentClick = useCallback((doc: OcrDocumentDTO) => {
    setSelectedImage(doc.path)
  }, [])

  return (
    <div className="space-y-4">
      {/* Header with scan button */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold">{t("ocr.title")}</h2>
          <p className="text-muted-foreground">{t("ocr.description")}</p>
        </div>
        <Button
          onClick={handleStartScan}
          disabled={scanning}
          variant="outline"
        >
          {scanning ? (
            <>
              <Loader2 className="h-4 w-4 mr-2 animate-spin" />
              {t("ocr.scanning")}
            </>
          ) : (
            <>
              <Play className="h-4 w-4 mr-2" />
              {t("ocr.scanButton")}
            </>
          )}
        </Button>
      </div>

      {/* Scan progress */}
      {scanning && scanStatus && (
        <div className="p-4 bg-muted rounded-lg">
          <div className="flex items-center gap-2">
            <Loader2 className="h-4 w-4 animate-spin" />
            <span className="text-sm">
              {t("ocr.filesProcessed", {
                count: scanStatus.filesProcessed,
                total: scanStatus.totalFiles,
              })}
            </span>
          </div>
          <p className="text-xs text-muted-foreground mt-1">{scanStatus.progress}</p>
        </div>
      )}

      {/* Document count */}
      {!scanning && totalDocuments > 0 && (
        <p className="text-sm text-muted-foreground">
          {totalDocuments === 1
            ? t("ocr.documentCountOne", { count: totalDocuments })
            : t("ocr.documentCount", { count: totalDocuments })}
        </p>
      )}

      {/* Loading state (initial) */}
      {isLoading && documents.length === 0 && (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin" />
        </div>
      )}

      {/* Empty state */}
      {!isLoading && !scanning && documents.length === 0 && (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <FileText className="h-12 w-12 text-muted-foreground mb-4" />
          <h3 className="text-lg font-medium">{t("ocr.empty")}</h3>
          <p className="text-sm text-muted-foreground mt-1">{t("ocr.emptyHint")}</p>
        </div>
      )}

      {/* Document grid */}
      {(!isLoading || documents.length > 0) && documents.length > 0 && (
        <>
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
            {documents.map((doc) => (
              <button
                key={doc.id}
                onClick={() => handleDocumentClick(doc)}
                className="group relative aspect-square rounded-lg overflow-hidden border bg-card hover:border-primary transition-colors"
              >
                {/* Thumbnail */}
                {doc.thumbnail ? (
                  <img
                    src={doc.thumbnail}
                    alt={doc.fileName}
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <div className="w-full h-full flex items-center justify-center bg-muted">
                    <FileText className="h-8 w-8 text-muted-foreground" />
                  </div>
                )}

                {/* Overlay on hover */}
                <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity p-2 flex flex-col justify-end">
                  <p className="text-xs text-white font-medium truncate">
                    {doc.fileName}
                  </p>
                  <div className="flex gap-2 mt-1">
                    <span className="text-[10px] text-white/80">
                      {t("ocr.angle")}: {doc.angle}°
                    </span>
                    <span className="text-[10px] text-white/80">
                      {t("ocr.confidence")}: {(doc.weightedConfidence * 100).toFixed(0)}%
                    </span>
                  </div>
                </div>
              </button>
            ))}
          </div>

          {/* Sentinel for infinite scroll */}
          <div ref={sentinelRef} className="h-4" />

          {/* Loading more indicator */}
          {isLoading && documents.length > 0 && (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-6 w-6 animate-spin" />
              <span className="ml-2 text-sm text-muted-foreground">
                {t("ocr.loadingMore")}
              </span>
            </div>
          )}

          {/* All loaded message */}
          {!hasMore && documents.length > 0 && (
            <p className="text-center text-sm text-muted-foreground py-4">
              {t("ocr.allLoaded", { count: totalDocuments })}
            </p>
          )}
        </>
      )}

      {/* Lightbox */}
      <OcrLightbox
        imagePath={selectedImage}
        onClose={() => setSelectedImage(null)}
      />
    </div>
  )
}
