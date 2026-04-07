import { Button } from "@/components/ui/button"
import { useSettings } from "@/providers/useSettings"
import { useTranslation } from "@/i18n"

export function LanguageToggle() {
  const { language, setLanguage } = useSettings()
  const { t } = useTranslation()

  const toggle = () => {
    setLanguage(language === "en" ? "ru" : "en")
  }

  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={toggle}
      title={t("header.toggleLanguage")}
      className="h-8 px-2 text-xs font-semibold text-white hover:bg-white/20"
    >
      {language === "en" ? "RU" : "EN"}
    </Button>
  )
}
