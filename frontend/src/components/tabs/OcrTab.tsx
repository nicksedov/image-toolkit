import { useState, useEffect, useCallback } from "react"
import { Button } from "@/components/ui/button"
import { FileText, Play, Loader2 } from "lucide-react"
import { useTranslation } from "@/i18n"
import { fetchOcrDocuments, startOcrClassification, fetchOcrClassificationStatus } from "@/api/endpoints"
import type { OcrDocumentDTO, OcrClassificationStatusResponse } from "@/types"
import { Pagination } from "@/components/pagination/Pagination"
import { OcrLightbox } from "@/components/gallery/OcrLightbox"
import { toast } from "sonner"

const PAGE_SIZE = 50

export function OcrTab() {
  const { t } = useTranslation()
  const [documents, setDocuments] = useState<OcrDocumentDTO[]>([])
  const [total, setTotal] = useState(0)
  const [currentPage, setCurrentPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [loading, setLoading] = useState(false)
  const [scanning, setScanning] = useState(false)
  const [scanStatus, setScanStatus] = useState<OcrClassificationStatusResponse | null>(null)
  const [selectedImage, setSelectedImage] = useState<string | null>(null)

  const loadDocuments = useCallback(async (page: number) => {
    setLoading(true)
    try {
      const resp = await fetchOcrDocuments(page, PAGE_SIZE)
      setDocuments(resp.documents)
      setTotal(resp.total)
      setCurrentPage(resp.currentPage)
      setTotalPages(resp.totalPages)
    } catch (err) {
      console.error("Failed to load OCR documents:", err)
    } finally {
      setLoading(false)
    }
  }, [])

  const checkScanStatus = useCallback(async () => {
    try {
      const status = await fetchOcrClassificationStatus()
      setScanStatus(status)
      setScanning(status.processing)
      if (!status.processing) {
        // Scan just finished, reload documents
        loadDocuments(currentPage)
      }
    } catch (err) {
      console.error("Failed to check scan status:", err)
    }
  }, [currentPage, loadDocuments])

  // Poll scan status when scanning
  useEffect(() => {
    if (!scanning) return

    checkScanStatus()
    const interval = setInterval(checkScanStatus, 2000)
    return () => clearInterval(interval)
  }, [scanning, checkScanStatus])

  // Load documents on mount and page change
  useEffect(() => {
    loadDocuments(currentPage)
  }, [currentPage, loadDocuments])

  const handleStartScan = async () => {
    try {
      await startOcrClassification()
      setScanning(true)
      toast.success(t("api.ocr.started"))
    } catch (err: any) {
      toast.error(err.message || t("api.ocr.failed"))
    }
  }

  const handleDocumentClick = useCallback((doc: OcrDocumentDTO) => {
    setSelectedImage(doc.path)
  }, [])

  const handlePageChange = (page: number) => {
    setCurrentPage(page)
    window.scrollTo({ top: 0, behavior: "smooth" })
  }

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
      {!scanning && total > 0 && (
        <p className="text-sm text-muted-foreground">
          {total === 1
            ? t("ocr.documentCountOne", { count: total })
            : t("ocr.documentCount", { count: total })}
        </p>
      )}

      {/* Loading state */}
      {loading && (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin" />
        </div>
      )}

      {/* Empty state */}
      {!loading && !scanning && documents.length === 0 && (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <FileText className="h-12 w-12 text-muted-foreground mb-4" />
          <h3 className="text-lg font-medium">{t("ocr.empty")}</h3>
          <p className="text-sm text-muted-foreground mt-1">{t("ocr.emptyHint")}</p>
        </div>
      )}

      {/* Document grid */}
      {!loading && documents.length > 0 && (
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

          {/* Pagination */}
          <Pagination
            currentPage={currentPage}
            totalPages={totalPages}
            hasPrevPage={currentPage > 1}
            hasNextPage={currentPage < totalPages}
            onPageChange={handlePageChange}
          />
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
