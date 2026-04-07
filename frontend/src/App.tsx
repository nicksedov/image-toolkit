import { useCallback, useEffect, useState } from "react"
import { Toaster } from "sonner"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Header } from "@/components/layout/Header"
import { SettingsTab } from "@/components/tabs/SettingsTab"
import { GalleryTab } from "@/components/tabs/GalleryTab"
import { DeduplicationTab } from "@/components/tabs/DeduplicationTab"
import { fetchFolders } from "@/api/endpoints"
import { Settings, ImageIcon, FileScan } from "lucide-react"

export default function App() {
  const [activeTab, setActiveTab] = useState<string>("deduplication")
  const [isCheckingGallery, setIsCheckingGallery] = useState(true)

  // On mount, check if gallery has folders. If not, force settings tab.
  useEffect(() => {
    async function checkGallery() {
      try {
        const result = await fetchFolders()
        if (result.totalFolders === 0) {
          setActiveTab("settings")
        }
      } catch {
        // If API fails, still allow normal navigation
      } finally {
        setIsCheckingGallery(false)
      }
    }
    checkGallery()
  }, [])

  const handleFolderAdded = useCallback(() => {
    // Gallery is no longer empty -- user can now switch tabs freely
  }, [])

  if (isCheckingGallery) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <div className="flex items-center justify-center py-20">
          <div className="text-sm text-muted-foreground">Loading...</div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background">
      <Header />

      <main className="mx-auto max-w-7xl px-4 py-4 sm:px-6">
        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList className="mb-4">
            <TabsTrigger value="settings" className="gap-1.5">
              <Settings className="h-3.5 w-3.5" />
              Settings
            </TabsTrigger>
            <TabsTrigger value="gallery" className="gap-1.5">
              <ImageIcon className="h-3.5 w-3.5" />
              Gallery
            </TabsTrigger>
            <TabsTrigger value="deduplication" className="gap-1.5">
              <FileScan className="h-3.5 w-3.5" />
              Deduplication
            </TabsTrigger>
          </TabsList>

          <TabsContent value="settings">
            <SettingsTab onFolderAdded={handleFolderAdded} />
          </TabsContent>

          <TabsContent value="gallery">
            <GalleryTab />
          </TabsContent>

          <TabsContent value="deduplication">
            <DeduplicationTab />
          </TabsContent>
        </Tabs>
      </main>

      <Toaster richColors position="top-right" />
    </div>
  )
}
