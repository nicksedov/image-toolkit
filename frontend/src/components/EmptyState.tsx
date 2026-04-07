import { ImageOff } from "lucide-react"

export function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <ImageOff className="h-16 w-16 text-muted-foreground mb-4" />
      <h2 className="text-xl font-semibold mb-2">No Duplicates Found</h2>
      <p className="text-muted-foreground max-w-md">
        Your media library appears to be clean, or you need to run a scan first.
      </p>
    </div>
  )
}
