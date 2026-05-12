import { AlertCircle, CheckCircle2, Info, Loader2, LucideIcon } from "lucide-react"

export type StatusAlertVariant = "info" | "success" | "warning" | "error" | "loading"

export interface StatusAlertProps {
  /** Alert variant */
  variant: StatusAlertVariant
  /** Message to display */
  message: string
  /** Custom icon (overrides variant default) */
  icon?: LucideIcon
  /** Custom class for container */
  className?: string
  /** Custom class for icon */
  iconClassName?: string
  /** Custom class for message */
  messageClassName?: string
  /** Whether to show icon */
  showIcon?: boolean
}

const variantConfig: Record<StatusAlertVariant, { icon: LucideIcon; containerClass: string; iconClass: string }> = {
  info: {
    icon: Info,
    containerClass: "bg-muted text-muted-foreground",
    iconClass: "text-muted-foreground",
  },
  success: {
    icon: CheckCircle2,
    containerClass: "bg-success/10 text-success",
    iconClass: "text-success",
  },
  warning: {
    icon: AlertCircle,
    containerClass: "bg-warning/10 text-warning",
    iconClass: "text-warning",
  },
  error: {
    icon: AlertCircle,
    containerClass: "bg-destructive/10 text-destructive",
    iconClass: "text-destructive",
  },
  loading: {
    icon: Loader2,
    containerClass: "bg-muted text-muted-foreground",
    iconClass: "text-muted-foreground",
  },
}

export function StatusAlert({
  variant,
  message,
  icon: CustomIcon,
  className = "flex items-start gap-2 p-3 rounded-lg text-sm",
  iconClassName,
  messageClassName = "",
  showIcon = true,
}: StatusAlertProps) {
  const config = variantConfig[variant]
  const Icon = CustomIcon || config.icon
  const isSpinning = variant === "loading"

  return (
    <div className={`${className} ${config.containerClass}`}>
      {showIcon && (
        <Icon
          className={`h-4 w-4 shrink-0 mt-0.5 ${isSpinning ? "animate-spin" : ""} ${iconClassName || config.iconClass}`}
        />
      )}
      <span className={messageClassName}>{message}</span>
    </div>
  )
}
