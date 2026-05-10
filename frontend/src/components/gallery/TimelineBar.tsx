import { useMemo } from "react"
import type { CalendarDateGroup, CalendarDateRange } from "@/types"

interface TimelineBarProps {
  dateRange: CalendarDateRange
  groups: CalendarDateGroup[]
  onNavigateToDate: (date: string) => void
}

const HEADER_HEIGHT = 56

export function TimelineBar({ dateRange, groups, onNavigateToDate }: TimelineBarProps) {
  const visibleDateRange = useMemo(() => {
    if (groups.length === 0) return { start: null, end: null }
    return {
      start: groups[0].date,
      end: groups[groups.length - 1].date,
    }
  }, [groups])

  if (!dateRange.minDate || !dateRange.maxDate || groups.length === 0) {
    return null
  }

  return (
    <div
      className="fixed right-0 w-16 z-10 hidden lg:flex flex-col justify-center"
      style={{
        pointerEvents: "none",
        top: `${HEADER_HEIGHT}px`,
        bottom: 0,
      }}
    >
      <div
        className="rounded-l-lg border-r border-y border-l-0 bg-gray-400/40 p-2 mx-0"
        style={{
          pointerEvents: "auto",
          height: "calc(100vh - 2rem)",
          maxHeight: "calc(100vh - 2rem)",
        }}
      >
        <div
          className="relative flex-1"
          style={{ height: "calc(100% - 2rem)" }}
          onClick={(e) => {
            const rect = e.currentTarget.getBoundingClientRect()
            const clickY = e.clientY - rect.top
            const clickPercent = clickY / rect.height

            const totalDays = daysBetween(dateRange.minDate, dateRange.maxDate)
            const targetOffset = Math.floor(clickPercent * totalDays)
            const targetDate = new Date(dateRange.minDate + "T00:00:00")
            targetDate.setDate(targetDate.getDate() + targetOffset)

            const dateStr = `${targetDate.getFullYear()}-${String(targetDate.getMonth() + 1).padStart(2, "0")}-${String(targetDate.getDate()).padStart(2, "0")}`

            onNavigateToDate(dateStr)
          }}
        >
          {/* Timeline track */}
          <div className="absolute left-1/2 -translate-x-1/2 w-0.5 h-full bg-muted" />

          {/* Year/Month scale markers */}
          {(() => {
            const markers = []
            const start = new Date(dateRange.minDate + "T00:00:00")
            const end = new Date(dateRange.maxDate + "T00:00:00")
            const totalDays = daysBetween(dateRange.minDate, dateRange.maxDate)

            // Generate year markers
            let current = new Date(start.getFullYear(), 0, 1)
            if (current < start) current = new Date(start.getFullYear() + 1, 0, 1)

            while (current <= end) {
              const offset = daysBetween(dateRange.minDate, current.toISOString().split("T")[0])
              const topPercent = totalDays > 0 ? (offset / totalDays) * 100 : 0

              markers.push(
                <div
                  key={`year-${current.getFullYear()}`}
                  className="absolute left-0 right-0 border-t border-primary/50"
                  style={{ top: `${topPercent}%` }}
                >
                  <span
                    className="absolute left-0 text-[9px] font-semibold text-foreground whitespace-nowrap -translate-x-1/2"
                    style={{ left: "50%" }}
                  >
                    {current.getFullYear()}
                  </span>
                </div>
              )

              current.setFullYear(current.getFullYear() + 1)
              current = new Date(current.getFullYear(), 0, 1)
            }

            return markers
          })()}

          {/* Visible range indicator */}
          {visibleDateRange.start && visibleDateRange.end && (
            (() => {
              const totalDays = daysBetween(dateRange.minDate, dateRange.maxDate)
              const startOffset = daysBetween(dateRange.minDate, visibleDateRange.start)
              const endOffset = daysBetween(dateRange.minDate, visibleDateRange.end)
              const topPercent = totalDays > 0 ? (startOffset / totalDays) * 100 : 0
              const heightPercent = totalDays > 0 ? ((endOffset - startOffset) / totalDays) * 100 : 0

              return (
                <div
                  className="absolute left-1/2 -translate-x-1/2 w-3 bg-blue-500/50 rounded-sm"
                  style={{
                    top: `${topPercent}%`,
                    height: `${Math.max(heightPercent, 5)}%`,
                  }}
                />
              )
            })()
          )}

          {/* Date markers for visible groups */}
          {groups.map((group) => {
            const offset = daysBetween(dateRange.minDate, group.date)
            const totalDays = daysBetween(dateRange.minDate, dateRange.maxDate)
            const topPercent = totalDays > 0 ? (offset / totalDays) * 100 : 0

            return (
              <div
                key={group.date}
                className="absolute left-1/2 -translate-x-1/2 w-2 h-2 rounded-full bg-primary/40 cursor-pointer hover:scale-150 transition-transform"
                style={{ top: `${topPercent}%` }}
                title={group.date}
                onClick={(e) => {
                  e.stopPropagation()
                  onNavigateToDate(group.date)
                }}
              />
            )
          })}
        </div>
      </div>
    </div>
  )
}

function daysBetween(date1: string, date2: string): number {
  const d1 = new Date(date1 + "T00:00:00")
  const d2 = new Date(date2 + "T00:00:00")
  return Math.floor((d2.getTime() - d1.getTime()) / (1000 * 60 * 60 * 24))
}
