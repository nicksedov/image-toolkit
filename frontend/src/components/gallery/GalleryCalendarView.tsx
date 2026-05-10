import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { fetchGalleryCalendar, fetchCalendarMonthInfo, fetchCalendarAllDates } from "@/api/endpoints"
import { Skeleton } from "@/components/ui/skeleton"
import { Calendar as CalendarIcon } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { GalleryImageDTO, CalendarDateGroup, CalendarDateRange, CalendarMonthInfo, TimelineDateMarker } from "@/types"
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
  const [allDates, setAllDates] = useState<TimelineDateMarker[]>([])
  const [loadedDates, setLoadedDates] = useState<Set<string>>(new Set()) // dates that have been loaded/visible

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

  const nextPageRef = useRef(1)
  const prevPageRef = useRef(0)
  const topSentinelRef = useRef<HTMLDivElement>(null)
  const bottomSentinelRef = useRef<HTMLDivElement>(null)
  const mainContentRef = useRef<HTMLDivElement>(null)
  const loadedPagesRef = useRef<Set<number>>(new Set()) // track which pages are already in groups
  const prevGroupsLengthRef = useRef(0) // track previous groups length for scroll restoration
  
  // Image preloading
  const preloadImageCache = useRef<Map<string, HTMLImageElement>>(new Map())

  const calendarMonthKey = useMemo(() => {
    const y = calendarViewDate.getFullYear()
    const m = calendarViewDate.getMonth() + 1
    return `${y}-${String(m).padStart(2, "0")}`
  }, [calendarViewDate])

  // Fetch calendar data - bidirectional loading
  const loadingRef = useRef(false)
  const loadPage = useCallback(async (page: number, direction: "append" | "prepend" | "reset" = "reset") => {
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

      if (direction === "reset") {
        setGroups(result.groups)
        loadedPagesRef.current = new Set([page])
      } else if (direction === "append") {
        setGroups((prev) => [...prev, ...result.groups])
        loadedPagesRef.current.add(page)
      } else {
        // prepend
        setGroups((prev) => [...result.groups, ...prev])
        loadedPagesRef.current.add(page)
      }
      setTotalImages(result.totalImages)
      setHasMore(result.hasMore)

      // Update date range on first load
      if (page === 1 || direction === "reset") {
        setDateRange(result.dateRange)
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

  // Restore scroll position when groups are prepended
  useEffect(() => {
    const prevLength = prevGroupsLengthRef.current
    const currentLength = groups.length
    
    // If groups increased and we loaded a previous page (detected by checking if first group changed)
    if (currentLength > prevLength && prevLength > 0) {
      // Scroll to maintain position - scroll down by approximate height of new content
      const scrollContainer = mainContentRef.current
      if (scrollContainer) {
        // Find the scrollable parent
        const scrollableParent = scrollContainer.parentElement?.closest('.overflow-y-auto') || window
        if ('scrollTop' in (scrollableParent as HTMLElement)) {
          // Approximate: each group has ~100px height, scroll down by the new groups
          const newGroupsCount = currentLength - prevLength
          const estimatedHeight = newGroupsCount * 150
          ;(scrollableParent as HTMLElement).scrollTop += estimatedHeight
        }
      }
    }
    
    prevGroupsLengthRef.current = currentLength
  }, [groups])

  // Initial load on mount
  useEffect(() => {
    const initialize = async () => {
      nextPageRef.current = 1
      prevPageRef.current = 0
      setGroups([])
      setInitialized(false)
      loadedPagesRef.current = new Set()
      await loadPage(1, "reset")
    }
    initialize()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []) // Only on mount

  // Fetch all dates for timeline markers on mount
  useEffect(() => {
    fetchCalendarAllDates()
      .then((result) => {
        setAllDates(result.dates)
      })
      .catch(() => {
        setAllDates([])
      })
  }, [])

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
      nextPageRef.current = 1
      prevPageRef.current = 0
      setGroups([])
      setInitialized(false)
      setDateRangeFilter({ start: null, end: null })
      setRangeSelecting(false)
      loadedPagesRef.current = new Set()
      await loadPage(1, "reset")
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
      
      nextPageRef.current = 1
      prevPageRef.current = 0
      setGroups([])
      setInitialized(false)
      loadedPagesRef.current = new Set()
      await loadPage(1, "reset")
    }
    initialize()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dateRangeFilter.start, dateRangeFilter.end])

  // Bottom sentinel - load next page when scrolling down
  useEffect(() => {
    const sentinel = bottomSentinelRef.current
    if (!sentinel) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasMore && !isLoading) {
          const page = nextPageRef.current
          if (!loadedPagesRef.current.has(page)) {
            loadPage(page, "append")
            nextPageRef.current = page + 1
          }
        }
      },
      { threshold: 0.1 }
    )

    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [hasMore, isLoading, loadPage])

  // Top sentinel - load previous page when scrolling up
  useEffect(() => {
    const sentinel = topSentinelRef.current
    if (!sentinel) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && prevPageRef.current > 0 && !isLoading) {
          const page = prevPageRef.current
          if (!loadedPagesRef.current.has(page)) {
            loadPage(page, "prepend")
            prevPageRef.current = page - 1
          }
        }
      },
      { threshold: 0.1 }
    )

    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [isLoading, loadPage])

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

  const handleNavigateToDate = async (date: string) => {
    // If the date is already loaded (visible in groups), just scroll to it
    if (loadedDates.has(date)) {
      const element = document.getElementById(`date-group-${date}`)
      if (element) {
        element.scrollIntoView({ behavior: "smooth", block: "start" })
      }
      return
    }

    // Find the page for this date from allDates
    const dateInfo = allDates.find((d) => d.date === date)
    if (!dateInfo) {
      setError(`No images found for date ${date}`)
      return
    }

    setIsLoading(true)
    setError(null)
    try {
      const targetPage = dateInfo.page

      // Load only the target page
      const result = await fetchGalleryCalendar(
        targetPage,
        PAGE_SIZE,
        dateRangeFilter.start ?? undefined,
        dateRangeFilter.end ?? undefined,
        calendarMonthKey
      )

      if (result.groups.length > 0) {
        setGroups(result.groups)
        setTotalImages(result.totalImages)
        setHasMore(result.hasMore)
        loadedPagesRef.current = new Set([targetPage])
        const newLoaded = new Set(loadedDates)
        result.groups.forEach((g) => newLoaded.add(g.date))
        setLoadedDates(newLoaded)

        // Set up bidirectional scroll: prev page above, next page below
        prevPageRef.current = targetPage - 1
        nextPageRef.current = targetPage + 1

        setTimeout(() => {
          const element = document.getElementById(`date-group-${date}`)
          if (element) {
            element.scrollIntoView({ behavior: "smooth", block: "start" })
          }
        }, 100)
      } else {
        setError(`No images found for date ${date}`)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load images")
    } finally {
      setIsLoading(false)
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
              {/* Top sentinel - for loading previous pages when scrolling up */}
              <div ref={topSentinelRef} className="h-4" />

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

              {isLoading && (
                <div className="flex justify-center py-4">
                  <div className="text-sm text-muted-foreground">{t("gallery.loadingMore")}</div>
                </div>
              )}

              {/* Bottom sentinel - for loading next pages when scrolling down */}
              <div ref={bottomSentinelRef} className="h-4" />

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
          allDates={allDates}
          dateRangeFilter={dateRangeFilter}
          loadedDates={loadedDates}
          onNavigateToDate={handleNavigateToDate}
        />
      </div>
    </div>
  )
}
