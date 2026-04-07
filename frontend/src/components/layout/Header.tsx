import { Badge } from "@/components/ui/badge"
import type { DuplicatesResponse } from "@/types"
import { FolderSearch, Files, FileImage } from "lucide-react"

interface HeaderProps {
  data: DuplicatesResponse | null
}

export function Header({ data }: HeaderProps) {
  return (
    <header className="border-b bg-gradient-to-r from-blue-600 to-indigo-700 text-white">
      <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6">
        <div className="flex flex-col gap-4">
          <div>
            <h1 className="text-2xl font-bold tracking-tight">Image Dedup</h1>
            <p className="text-sm text-blue-100">
              Manage duplicate images in your media library
            </p>
          </div>

          {data && (
            <>
              <div className="flex flex-wrap gap-4">
                <StatCard
                  icon={<FolderSearch className="h-4 w-4" />}
                  label="Duplicate Groups"
                  value={data.totalGroups}
                />
                <StatCard
                  icon={<Files className="h-4 w-4" />}
                  label="Total Duplicate Files"
                  value={data.totalFiles}
                />
                <StatCard
                  icon={<FileImage className="h-4 w-4" />}
                  label="Files on This Page"
                  value={data.pageFiles}
                />
              </div>

              {data.scannedDirs.length > 0 && (
                <div className="flex flex-wrap items-center gap-2 text-sm text-blue-100">
                  <span>Scanned:</span>
                  {data.scannedDirs.map((dir) => (
                    <Badge key={dir} variant="secondary" className="bg-white/20 text-white border-white/30 text-xs font-mono">
                      {dir}
                    </Badge>
                  ))}
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </header>
  )
}

function StatCard({ icon, label, value }: { icon: React.ReactNode; label: string; value: number }) {
  return (
    <div className="flex items-center gap-2 rounded-lg bg-white/10 px-4 py-2 backdrop-blur-sm">
      {icon}
      <div>
        <div className="text-xl font-bold">{value.toLocaleString()}</div>
        <div className="text-xs text-blue-100">{label}</div>
      </div>
    </div>
  )
}
