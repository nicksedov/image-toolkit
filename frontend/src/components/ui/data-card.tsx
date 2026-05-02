import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { cn } from "@/lib/utils"
import type { LucideIcon } from "lucide-react"

interface DataCardProps {
  /** Card title */
  title: string
  /** Card description */
  description?: string
  /** Optional icon displayed next to the title */
  icon?: LucideIcon
  /** Optional actions rendered on the right side of header */
  actions?: React.ReactNode
  /** Card content */
  children: React.ReactNode
  /** Optional footer content */
  footer?: React.ReactNode
  /** Additional classes for the card */
  className?: string
  /** Additional classes for the header */
  headerClassName?: string
  /** Additional classes for the content */
  contentClassName?: string
}

/**
 * Reusable card component with header, optional icon, description, actions, and footer.
 */
export function DataCard({
  title,
  description,
  icon: Icon,
  actions,
  children,
  footer,
  className,
  headerClassName,
  contentClassName,
}: DataCardProps) {
  return (
    <Card className={cn(className)}>
      <CardHeader className={cn("flex flex-row items-start justify-between space-y-0", headerClassName)}>
        <div className="space-y-1">
          <CardTitle className="flex items-center gap-2">
            {Icon && <Icon className="h-5 w-5" />}
            {title}
          </CardTitle>
          {description && <CardDescription>{description}</CardDescription>}
        </div>
        {actions && <div className="flex items-center gap-2">{actions}</div>}
      </CardHeader>
      <CardContent className={cn(contentClassName)}>{children}</CardContent>
      {footer && (
        <div className="border-t px-6 py-3">{footer}</div>
      )}
    </Card>
  )
}
