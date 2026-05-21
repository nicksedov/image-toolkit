import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { Settings, Brain } from "lucide-react"
import { useTranslation } from "@/i18n"
import { AdminGeneralTab } from "./AdminGeneralTab"
import { AdminAnalysisTab } from "./AdminAnalysisTab"

export function AdminSettingsTab() {
  const { t } = useTranslation()

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold mb-1">{t("adminPanel.adminSettings")}</h2>
        <p className="text-sm text-muted-foreground">
          {t("adminPanel.adminSettingsDescription")}
        </p>
      </div>

      <Tabs defaultValue="general" className="w-full">
        <TabsList className="grid w-full grid-cols-2">
          <TabsTrigger value="general" className="flex items-center gap-2">
            <Settings className="h-4 w-4" />
            {t("adminSettings.tabs.general")}
          </TabsTrigger>
          <TabsTrigger value="analysis" className="flex items-center gap-2">
            <Brain className="h-4 w-4" />
            {t("adminSettings.tabs.analysis")}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="general" className="mt-6">
          <AdminGeneralTab />
        </TabsContent>

        <TabsContent value="analysis" className="mt-6">
          <AdminAnalysisTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}
