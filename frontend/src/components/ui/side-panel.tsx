import { ReactNode } from "react"

export interface SidePanelProps {
  /** Header content (fixed at top) */
  header: ReactNode
  /** Content area (scrollable) */
  children: ReactNode
  /** Width classes (responsive) */
  widthClassName?: string
  /** Whether to show border */
  showBorder?: boolean
  /** Border position */
  borderPosition?: "left" | "right" | "top" | "bottom"
  /** Background color */
  bgClassName?: string
  /** Custom class for header */
  headerClassName?: string
  /** Custom class for content */
  contentClassName?: string
  /** Custom class for container */
  className?: string
}

export function SidePanel({
  header,
  children,
  widthClassName = "w-full md:w-[400px] lg:w-[450px] md:min-w-[350px]",
  showBorder = true,
  borderPosition = "left",
  bgClassName = "bg-card",
  headerClassName = "p-4 flex-shrink-0",
  contentClassName = "flex-1 min-h-0 overflow-y-auto",
  className = "shrink-0 flex flex-col max-h-[90vh]",
}: SidePanelProps) {
  const borderClasses: Record<string, string> = {
    left: "border-l",
    right: "border-r",
    top: "border-t",
    bottom: "border-b",
  }

  return (
    <div className={`${widthClassName} ${bgClassName} ${showBorder ? borderClasses[borderPosition] : ""} ${className}`}>
      <div className={headerClassName}>{header}</div>
      <div className={contentClassName}>
        {children}
      </div>
    </div>
  )
}
