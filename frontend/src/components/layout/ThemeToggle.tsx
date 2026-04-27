import { Moon, Sun } from "lucide-react"
import { useSettings } from "@/providers/useSettings"
import { useTranslation } from "@/i18n"
import { ThemeSelect, ThemeSelectContent, ThemeSelectItem, ThemeSelectTrigger, ThemeSelectValue } from "@/components/ui/theme-select"
import type { Theme } from "@/theme"

export function ThemeToggle() {
  const { theme, setTheme } = useSettings()
  const { t } = useTranslation()

  const themeColors: Record<Theme, { label: string; icon: React.ReactNode; color: string }> = {
    "light-purple": { 
      label: "☀️ Светло-фиолетовая", 
      icon: <Sun className="h-4 w-4" />, 
      color: "bg-purple-100" 
    },
    "dark-purple": { 
      label: "🌙 Темно-фиолетовая", 
      icon: <Moon className="h-4 w-4" />, 
      color: "bg-purple-900" 
    },
    "light-green": { 
      label: "🌿 Светло-зеленая", 
      icon: <Sun className="h-4 w-4" />, 
      color: "bg-green-100" 
    },
    "dark-green": { 
      label: "🌲 Темно-зеленая", 
      icon: <Moon className="h-4 w-4" />, 
      color: "bg-green-900" 
    },
    "light-blue": { 
      label: "🌊 Светло-синяя", 
      icon: <Sun className="h-4 w-4" />, 
      color: "bg-blue-100" 
    },
    "dark-blue": { 
      label: "🌌 Темно-синяя", 
      icon: <Moon className="h-4 w-4" />, 
      color: "bg-blue-900" 
    },
    "light-orange": { 
      label: "🌅 Светло-оранжевая", 
      icon: <Sun className="h-4 w-4" />, 
      color: "bg-orange-100" 
    },
    "dark-orange": { 
      label: "🍊 Темно-оранжевая", 
      icon: <Moon className="h-4 w-4" />, 
      color: "bg-orange-900" 
    },
    "dark-contrast": { 
      label: "⚫ Dark Contrast", 
      icon: <Moon className="h-4 w-4" />, 
      color: "bg-gray-900" 
    },
  }

  return (
    <div className="flex items-center gap-2">
      <ThemeSelect value={theme} onValueChange={(value: Theme) => setTheme(value)}>
        <ThemeSelectTrigger
          title={t("header.toggleTheme")}
          className="h-8 w-8 p-0 text-white hover:bg-white/20"
        >
          <ThemeSelectValue placeholder={t("header.toggleTheme")} />
        </ThemeSelectTrigger>
        <ThemeSelectContent>
          {Object.entries(themeColors).map(([key, themeData]) => (
            <ThemeSelectItem 
              key={key} 
              value={key}
              className="flex items-center gap-2"
            >
              <div className={`h-3 w-3 rounded-full ${themeData.color}`} />
              <span>{themeData.label}</span>
            </ThemeSelectItem>
          ))}
        </ThemeSelectContent>
      </ThemeSelect>
    </div>
  )
}
