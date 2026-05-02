import { useTranslation } from "@/i18n"
import { useAuth } from "@/providers/AuthProvider"
import { LogOut, User } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { IconButton } from "@/components/ui/icon-button"

export function Header() {
  const { t } = useTranslation()
  const { user, logout } = useAuth()

  return (
    <header className="sticky top-0 z-10 border-b bg-header px-6 py-3">
      <div className="flex items-center justify-end gap-3">
        {user && (
          <>
            <div className="flex items-center gap-2">
              <User className="h-4 w-4 text-muted-foreground" />
              <span className="text-sm font-medium">{user.displayName}</span>
              <Badge variant="outline" className="text-xs">
                {user.role === "admin" ? t("adminPanel.roleAdmin") : t("adminPanel.roleUser")}
              </Badge>
            </div>
            <IconButton
              variant="ghost"
              size="sm"
              icon={LogOut}
              onClick={logout}
            >
              {t("adminPanel.logout")}
            </IconButton>
          </>
        )}
      </div>
    </header>
  )
}
