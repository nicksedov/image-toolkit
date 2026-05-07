import { useMemo } from "react"
import { useTranslation } from "@/i18n"
import { ChevronLeft, ChevronRight } from "lucide-react"
import type { CalendarDateRange, CalendarMonthInfo } from "@/types"

const MONTHS = [
  { value: 0, label: "Jan" },
  { value: 1, label: "Feb" },
  { value: 2, label: "Mar" },
  { value: 3, label: "Apr" },
  { value: 4, label: "May" },
  { value: 5, label: "Jun" },
  { value: 6, label: "Jul" },
  { value: 7, label: "Aug" },
  { value: 8, label: "Sep" },
  { value: 9, label: "Oct" },
  { value: 10, label: "Nov" },
  { value: 11, label: "Dec" },
]

interface CalendarWidgetProps {
  dateRange: CalendarDateRange
  monthInfo: CalendarMonthInfo | null
  dayCounts: Map<number, number>
  calendarViewDate: Date
  dateRangeFilter: { start: string | null; end: string | null }
  rangeSelecting: boolean
  onMonthChange: (date: Date) => void
  onDateSelect: (date: string) => void
  onDateRangeSelect: (start: string, end: string) => void
  onClearFilter: () => void
}

export function CalendarWidget({
  dateRange,
  monthInfo,
  dayCounts,
  calendarViewDate,
  dateRangeFilter,
  rangeSelecting,
  onMonthChange,
  onDateSelect,
  onDateRangeSelect,
  onClearFilter,
}: CalendarWidgetProps) {
  const { t } = useTranslation()

  const prevMonth = () => {
    onMonthChange(new Date(calendarViewDate.getFullYear(), calendarViewDate.getMonth() - 1, 1))
  }

  const nextMonth = () => {
    onMonthChange(new Date(calendarViewDate.getFullYear(), calendarViewDate.getMonth() + 1, 1))
  }

  const handleMonthChange = (newMonth: number) => {
    onMonthChange(new Date(calendarViewDate.getFullYear(), newMonth, 1))
  }

  const handleYearChange = (newYear: number) => {
    onMonthChange(new Date(newYear, calendarViewDate.getMonth(), 1))
  }

  const isInSelectedRange = (date: string) => {
    if (!dateRangeFilter.start) return false
    if (!dateRangeFilter.end) return date === dateRangeFilter.start
    return date >= dateRangeFilter.start && date <= dateRangeFilter.end
  }

  const selectFullMonth = () => {
    const year = calendarViewDate.getFullYear()
    const month = calendarViewDate.getMonth()
    const firstDay = `${year}-${String(month + 1).padStart(2, "0")}-01`
    const lastDayDate = new Date(year, month + 1, 0)
    const lastDay = `${year}-${String(month + 1).padStart(2, "0")}-${String(lastDayDate.getDate()).padStart(2, "0")}`
    
    onDateRangeSelect(firstDay, lastDay)
  }

  // Generate all days of the month as a scrollable strip
  const calendarDays = useMemo(() => {
    const year = calendarViewDate.getFullYear()
    const month = calendarViewDate.getMonth()
    const lastDay = new Date(year, month + 1, 0)
    const daysInMonth = lastDay.getDate()

    const daysWithImages = new Set(monthInfo?.days ?? [])

    const days: { date: string; day: number; hasImages: boolean; imageCount: number }[] = []
    for (let d = 1; d <= daysInMonth; d++) {
      const dateStr = `${year}-${String(month + 1).padStart(2, "0")}-${String(d).padStart(2, "0")}`
      const count = dayCounts.get(d) ?? 0
      days.push({
        date: dateStr,
        day: d,
        hasImages: daysWithImages.has(d),
        imageCount: count,
      })
    }

    return days
  }, [calendarViewDate, monthInfo, dayCounts])

  return (
    <div className="rounded-lg border bg-card p-3">
      {/* Month/Year selector */}
      <div className="flex items-center justify-between mb-2 gap-2">
        <button onClick={prevMonth} className="p-1 hover:bg-muted rounded flex-shrink-0">
          <ChevronLeft className="h-4 w-4" />
        </button>
        
        <div className="flex items-center gap-2">
          {/* Month dropdown */}
          <select
            value={calendarViewDate.getMonth()}
            onChange={(e) => handleMonthChange(parseInt(e.target.value))}
            className="text-sm font-medium bg-background dark:bg-zinc-800 text-foreground dark:text-zinc-100 border border-border rounded px-2 py-1 outline-none cursor-pointer"
          >
            {MONTHS.map((m) => (
              <option key={m.value} value={m.value} className="bg-background dark:bg-zinc-800 text-foreground dark:text-zinc-100">
                {new Date(2000, m.value, 1).toLocaleDateString(undefined, { month: "long" })}
              </option>
            ))}
          </select>

          {/* Year dropdown */}
          <select
            value={calendarViewDate.getFullYear()}
            onChange={(e) => handleYearChange(parseInt(e.target.value))}
            className="text-sm font-medium bg-background dark:bg-zinc-800 text-foreground dark:text-zinc-100 border border-border rounded px-2 py-1 outline-none cursor-pointer"
          >
            {(() => {
              // Generate year range from dateRange or fallback to current year ±5
              const currentYear = calendarViewDate.getFullYear()
              let startYear = currentYear - 5
              let endYear = currentYear + 5
              
              // Use actual data range if available
              if (dateRange.minDate && dateRange.maxDate) {
                const minYear = new Date(dateRange.minDate + "T00:00:00").getFullYear()
                const maxYear = new Date(dateRange.maxDate + "T00:00:00").getFullYear()
                startYear = minYear
                endYear = maxYear
              }
              
              const years = []
              for (let y = startYear; y <= endYear; y++) {
                years.push(y)
              }
              
              return years.map((year) => (
                <option key={year} value={year} className="bg-background dark:bg-zinc-800 text-foreground dark:text-zinc-100">
                  {year}
                </option>
              ))
            })()}
          </select>
        </div>

        <button onClick={nextMonth} className="p-1 hover:bg-muted rounded flex-shrink-0">
          <ChevronRight className="h-4 w-4" />
        </button>
      </div>

      {/* Horizontal scrollable day strip */}
      <div className="flex gap-0.75 overflow-x-auto pb-1 pt-2 px-1 calendar-days-no-scrollbar">
        {calendarDays.map((day) => {
          const isSelected = isInSelectedRange(day.date)
          const isRangeStart = day.date === dateRangeFilter.start
          const isRangeEnd = day.date === dateRangeFilter.end && dateRangeFilter.end !== null
          
          return (
            <button
              key={day.date}
              disabled={!day.date}
              className={`
                flex-shrink-0 w-9 h-9 flex flex-col items-center justify-center text-xs rounded-md
                transition-all relative
                ${isSelected
                  ? "bg-primary text-primary-foreground hover:bg-primary/90 font-medium cursor-pointer"
                  : day.hasImages 
                    ? "bg-emerald-100 dark:bg-emerald-900/30 hover:bg-emerald-200 dark:hover:bg-emerald-900/50 text-emerald-700 dark:text-emerald-200 font-medium cursor-pointer"
                    : "bg-red-50 dark:bg-red-900/20 text-muted-foreground/70 dark:text-muted-foreground/80 hover:bg-red-100 dark:hover:bg-red-900/30"
                }
                ${isRangeStart || isRangeEnd ? "ring-2 ring-primary ring-offset-2" : ""}
                ${isSelected && !isRangeStart && !isRangeEnd ? "opacity-80" : ""}
              `}
              onClick={() => day.date && onDateSelect(day.date)}
              title={day.hasImages ? `${day.imageCount} ${day.imageCount === 1 ? "image" : "images"}` : "No images"}
            >
              <span className="text-[11px] leading-none">{day.day}</span>
            </button>
          )
        })}
      </div>

      {/* Date filter controls */}
      <div className="mt-2 pt-2 border-t flex items-center justify-between">
        <button
          onClick={selectFullMonth}
          className="text-xs px-2 py-1 bg-primary text-primary-foreground rounded hover:bg-primary/90 transition-colors"
        >
          {t("gallery.calendar.fullMonth")}
        </button>
        
        {(dateRangeFilter.start || dateRangeFilter.end) && (
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground">
              {rangeSelecting
                ? `Selecting: ${dateRangeFilter.start}${dateRangeFilter.end ? ` — ${dateRangeFilter.end}` : " (click end date)"}`
                : dateRangeFilter.start === dateRangeFilter.end
                  ? dateRangeFilter.start
                  : `${dateRangeFilter.start} — ${dateRangeFilter.end}`}
            </span>
            <button
              onClick={onClearFilter}
              className="text-xs text-primary hover:underline"
            >
              {t("gallery.calendar.clearFilter")}
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
