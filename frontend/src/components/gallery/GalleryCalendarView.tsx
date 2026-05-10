import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { fetchGalleryCalendar, fetchCalendarMonthInfo } from "@/api/endpoints"
import { Skeleton } from "@/components/ui/skeleton"
import { Calendar as CalendarIcon } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { GalleryImageDTO, CalendarDateGroup, CalendarDateRange, CalendarMonthInfo } from "@/types"
import { CalendarImageGrid } from "./CalendarImageGrid"
import { CalendarWidget } from "./CalendarWidget"
import { TimelineBar } from "./TimelineBar"

interface GalleryCalendarViewProps {
  onImageClick: (image: GalleryImageDTO) => void
  onImageView?: (image: GalleryImageDTO) => void
  onImageOcr?: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO, removeThumbnail: () => void) => void
}

const PAGE_SIZE = 50

export function GalleryCalendarView({ onImageClick, onImageView, onImageOcr, onImageDownload, onImageDelete }: GalleryCalendarViewProps) {
  const { t } = useTranslation()

  const [groups, setGroups] = useState<CalendarDateGroup[]>([])
  const [totalImages, setTotalImages] = useState(0)
  const [hasMore, setHasMore] = useState(true)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [initialized, setInitialized] = useState(false)
  const [dateRange, setDateRange] = useState<CalendarDateRange>({ minDate: "", maxDate: "", totalWithDate: 0 })
  const [monthInfo, setMonthInfo] = useState<CalendarMonthInfo | null>(null)
  const [dayCounts, setDayCounts] = useState<Map<number, number>>(new Map()) // day -> count

  // Date filter state - now supports range selection
  const [dateRangeFilter, setDateRangeFilter] = useState<{ start: string | null; end: string | null }>({
    start: null,
    end: null,
  })
  const [rangeSelecting, setRangeSelecting] = useState(false)

  // Calendar widget state
  const [calendarViewDate, setCalendarViewDate] = useState(() => {
    return new Date()
  })

  const pageRef = useRef(1)
  const prefetchedPageRef = useRef(0)
  const sentinelRef = useRef<HTMLDivElement>(null)
  const mainContentRef = useRef<HTMLDivElement>(null)
  
  // Image preloading
  const preloadImageCache = useRef<Map<string, HTMLImageElement>>(new Map())

  const calendarMonthKey = useMemo(() => {
    const y = calendarViewDate.getFullYear()
    const m = calendarViewDate.getMonth() + 1
    return `${y}-${String(m).padStart(2, "0")}`
  }, [calendarViewDate])

  // Fetch calendar data
  const loadingRef = useRef(false)
  const loadPage = useCallback(async (page: number, reset = false) => {
    if (loadingRef.current) return
    loadingRef.current = true
    setIsLoading(true)
    setError(null)
    try {
      const result = await fetchGalleryCalendar(
        page,
        PAGE_SIZE,
        dateRangeFilter.start ?? undefined,
        dateRangeFilter.end ?? undefined,
        calendarMonthKey
      )

      if (reset) {
        setGroups(result.groups)
      } else {
        setGroups((prev) => [...prev, ...result.groups])
      }
      setTotalImages(result.totalImages)
      setHasMore(result.hasMore)

      // Update date range on first load
      if (page === 1) {
        setDateRange(result.dateRange)
        // Set calendar to the month of the oldest image (minDate) only on initial load
        // when no calendar month has been explicitly selected by the user
        if (!dateRangeFilter.start && !initialized && result.dateRange.minDate) {
          const minDate = new Date(result.dateRange.minDate + "T00:00:00")
          setCalendarViewDate(new Date(minDate.getFullYear(), minDate.getMonth(), 1))
        }
      }

      // Update month info
      if (result.months.length > 0) {
        setMonthInfo(result.months[0])
      }

      setInitialized(true)
      pageRef.current = page + 1
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load images")
    } finally {
      loadingRef.current = false
      setIsLoading(false)
    }
  }, [dateRangeFilter.start, dateRangeFilter.end, calendarMonthKey, initialized])

  // Preload images for smoother scrolling
  const preloadImages = useCallback((imageUrls: string[]) => {
    const MAX_CACHE_SIZE = 100
    
    imageUrls.forEach((url) => {
      if (!url || preloadImageCache.current.has(url)) return
      
      // Limit cache size
      if (preloadImageCache.current.size >= MAX_CACHE_SIZE) {
        const firstKey = preloadImageCache.current.keys().next().value
        if (firstKey) {
          preloadImageCache.current.delete(firstKey)
        }
      }
      
      const img = new Image()
      img.src = url
      preloadImageCache.current.set(url, img)
    })
  }, [])

  // Preload images when groups change
  useEffect(() => {
    const imageUrls = groups.flatMap((group) => 
      group.images.map((img) => img.thumbnail).filter(Boolean) as string[]
    )
    
    // Preload with slight delay to not block initial render
    const timer = setTimeout(() => {
      preloadImages(imageUrls)
    }, 100)
    
    return () => clearTimeout(timer)
  }, [groups, preloadImages])

  // Preload next page images when approaching end of current content
  useEffect(() => {
    if (!hasMore) return
    
    const checkAndPreloadNextPage = () => {
      if (!mainContentRef.current || isLoading) return
      
      const scrollHeight = mainContentRef.current.scrollHeight
      const scrollTop = mainContentRef.current.scrollTop || window.scrollY
      const clientHeight = window.innerHeight
      
      // If user has scrolled past 50% of content, preload next page (once per page)
      if (scrollTop + clientHeight > scrollHeight * 0.5) {
        const nextPage = pageRef.current
        
        // Only prefetch if this page hasn't been prefetched yet
        if (nextPage <= prefetchedPageRef.current) return
        
        prefetchedPageRef.current = nextPage
        fetchGalleryCalendar(
          nextPage,
          PAGE_SIZE,
          dateRangeFilter.start ?? undefined,
          dateRangeFilter.end ?? undefined,
          calendarMonthKey
        )
          .then((result) => {
            const imageUrls = result.groups.flatMap((group) =>
              group.images.map((img) => img.thumbnail).filter(Boolean) as string[]
            )
            preloadImages(imageUrls)
          })
          .catch(() => {
            // Silently fail - next page will load normally when needed
            prefetchedPageRef.current = nextPage - 1 // allow retry on failure
          })
      }
    }
    
    window.addEventListener("scroll", checkAndPreloadNextPage, { passive: true })
    return () => window.removeEventListener("scroll", checkAndPreloadNextPage)
  }, [hasMore, isLoading, dateRangeFilter.start, dateRangeFilter.end, calendarMonthKey, preloadImages])

  // Initial load on mount
  useEffect(() => {
    const initialize = async () => {
      pageRef.current = 1
      prefetchedPageRef.current = 0
      setGroups([])
      setInitialized(false)
      await loadPage(1, true)
    }
    initialize()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []) // Only on mount

  // Load month info when calendar month changes (using lightweight endpoint)
  useEffect(() => {
    fetchCalendarMonthInfo(calendarMonthKey)
      .then((result) => {
        setMonthInfo({ year: result.year, month: result.month, days: result.days })
        // Build day count map
        const countMap = new Map<number, number>()
        result.dayCounts.forEach((dc) => {
          countMap.set(dc.day, dc.count)
        })
        setDayCounts(countMap)
      })
      .catch(() => {
        // Reset on error
        setMonthInfo(null)
        setDayCounts(new Map())
      })
  }, [calendarMonthKey])

  // Reload calendar data with thumbnails when month/year changes
  useEffect(() => {
    const initialize = async () => {
      pageRef.current = 1
      prefetchedPageRef.current = 0
      setGroups([])
      setInitialized(false)
      setDateRangeFilter({ start: null, end: null })
      setRangeSelecting(false)
      await loadPage(1, true)
    }
    initialize()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [calendarMonthKey])

  // Reload when date range filter changes (user clicking on calendar days)
  // Only triggers when start/end are actual dates, not null resets from month changes
  useEffect(() => {
    const initialize = async () => {
      // Skip if both are null (this means it was reset by month/year change, handled above)
      if (dateRangeFilter.start === null && dateRangeFilter.end === null) return
      // Skip if not yet initialized
      if (!initialized) return
      
      pageRef.current = 1
      prefetchedPageRef.current = 0
      setGroups([])
      setInitialized(false)
      await loadPage(1, true)
    }
    initialize()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dateRangeFilter.start, dateRangeFilter.end])

  // Infinite scroll
  useEffect(() => {
    const sentinel = sentinelRef.current
    if (!sentinel) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasMore && !isLoading) {
          loadPage(pageRef.current)
        }
      },
      { threshold: 0.1 }
    )

    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [hasMore, isLoading, loadPage])

  const handleDateSelect = (date: string) => {
    if (!rangeSelecting) {
      // Start range selection
      setDateRangeFilter({ start: date, end: null })
      setRangeSelecting(true)
    } else {
      // Complete range selection
      if (dateRangeFilter.start) {
        const startDate = dateRangeFilter.start
        // Ensure start <= end
        if (date >= startDate) {
          setDateRangeFilter({ start: startDate, end: date })
        } else {
          setDateRangeFilter({ start: date, end: startDate })
        }
      } else {
        setDateRangeFilter({ start: date, end: date })
      }
      setRangeSelecting(false)
    }
  }

  const handleDateRangeSelect = (start: string, end: string) => {
    // Directly set the complete date range
    setDateRangeFilter({ start, end })
    setRangeSelecting(false)
  }

  const clearDateRangeFilter = () => {
    setDateRangeFilter({ start: null, end: null })
    setRangeSelecting(false)
  }

  const handleNavigateToDate = (date: string) => {
    const element = document.getElementById(`date-group-${date}`)
    if (element) {
      element.scrollIntoView({ behavior: "smooth", block: "start" })
    }
  }

  return (
    <div className="space-y-4">
      {/* Header with image count */}
      <div className="flex items-center gap-2">
        <CalendarIcon className="h-5 w-5 text-muted-foreground" />
        <span className="text-sm text-muted-foreground">
          {dateRange.totalWithDate > 0
            ? (dateRange.totalWithDate === 1
              ? t("gallery.imageCountOne", { count: dateRange.totalWithDate.toLocaleString() })
              : t("gallery.imageCount", { count: dateRange.totalWithDate.toLocaleString() }))
            : t("gallery.calendar.noDateInfo")
          }
        </span>
      </div>

      {/* Horizontal Calendar Widget */}
      <CalendarWidget
        dateRange={dateRange}
        monthInfo={monthInfo}
        dayCounts={dayCounts}
        calendarViewDate={calendarViewDate}
        dateRangeFilter={dateRangeFilter}
        rangeSelecting={rangeSelecting}
        onMonthChange={setCalendarViewDate}
        onDateSelect={handleDateSelect}
        onDateRangeSelect={handleDateRangeSelect}
        onClearFilter={clearDateRangeFilter}
      />

      {/* Main content area with images and timeline */}
      <div className="flex gap-4" style={{ position: "relative" }}>
        {/* Images area */}
        <div ref={mainContentRef} className="flex-1 min-w-0">
          {error && (
            <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
              {error}
            </div>
          )}

          {!initialized && isLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <Skeleton key={i} className="h-40 w-full rounded-lg" />
              ))}
            </div>
          ) : groups.length === 0 && !isLoading ? (
            <div className="rounded-lg border border-dashed p-12 text-center">
              <CalendarIcon className="mx-auto h-10 w-10 text-muted-foreground/50" />
              <p className="mt-2 text-sm font-medium text-muted-foreground">
                {dateRangeFilter.start ? t("gallery.calendar.noImagesForDate") : t("gallery.calendar.noDateInfo")}
              </p>
              <p className="text-xs text-muted-foreground/70">
                {dateRangeFilter.start ? t("gallery.calendar.clearFilterHint") : t("gallery.calendar.noDateInfoHint")}
              </p>
            </div>
          ) : (
            <>
              <CalendarImageGrid
                groups={groups}
                onImageClick={onImageClick}
                onImageView={onImageView}
                onImageOcr={onImageOcr}
                onImageDownload={onImageDownload}
                onImageDelete={(image) => {
                  onImageDelete?.(image, () => {
                    setGroups((prev) =>
                      prev
                        .map((g) => ({ ...g, images: g.images.filter((img) => img.id !== image.id) }))
                        .map((g) => ({ ...g, imageCount: g.images.length }))
                    )
                    setTotalImages((prev) => Math.max(0, prev - 1))
                  })
                }}
              />

              <div ref={sentinelRef} className="h-4" />

              {isLoading && (
                <div className="flex justify-center py-4">
                  <div className="text-sm text-muted-foreground">{t("gallery.loadingMore")}</div>
                </div>
              )}

              {!hasMore && groups.length > 0 && (
                <div className="text-center text-xs text-muted-foreground py-4">
                  {t("gallery.allLoaded", { count: totalImages.toLocaleString() })}
                </div>
              )}
            </>
          )}
        </div>

        {/* Timeline sidebar */}
        <TimelineBar
          dateRange={dateRange}
          groups={groups}
          onNavigateToDate={handleNavigateToDate}
        />
      </div>
    </div>
  )
}
