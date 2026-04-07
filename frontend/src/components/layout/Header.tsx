import { useTranslation } from "@/i18n"
import { ThemeToggle } from "./ThemeToggle"
import { LanguageToggle } from "./LanguageToggle"

export function Header() {
  const { t } = useTranslation()

  return (
    <header className="border-b bg-gradient-to-r from-blue-600 to-indigo-700 text-white">
      <div className="mx-auto max-w-7xl px-4 py-4 sm:px-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("header.title")}</h1>
          <p className="text-sm text-blue-100">
            {t("header.subtitle")}
          </p>
        </div>
        <div className="flex items-center gap-1">
          <ThemeToggle />
          <LanguageToggle />
        </div>
      </div>
    </header>
  )
}
