import { Button } from "@/components/ui/button"
import { Moon, Sun } from "lucide-react"
import { useSettings } from "@/providers/useSettings"
import { useTranslation } from "@/i18n"

export function ThemeToggle() {
  const { theme, toggleTheme } = useSettings()
  const { t } = useTranslation()

  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={toggleTheme}
      title={t("header.toggleTheme")}
      className="h-8 w-8 p-0 text-white hover:bg-white/20"
    >
      {theme === "dark" ? (
        <Sun className="h-4 w-4" />
      ) : (
        <Moon className="h-4 w-4" />
      )}
    </Button>
  )
}
