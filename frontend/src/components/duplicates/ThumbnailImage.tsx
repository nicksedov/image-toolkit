import { Skeleton } from "@/components/ui/skeleton"
import { ImageOff } from "lucide-react"

interface ThumbnailImageProps {
  src: string
  isLoading?: boolean
}

export function ThumbnailImage({ src, isLoading }: ThumbnailImageProps) {
  if (isLoading) {
    return <Skeleton className="h-32 w-32 rounded-md" />
  }

  if (!src) {
    return (
      <div className="flex h-32 w-32 items-center justify-center rounded-md bg-muted">
        <ImageOff className="h-8 w-8 text-muted-foreground" />
      </div>
    )
  }

  return (
    <img
      src={src}
      alt="Thumbnail"
      className="h-32 w-32 rounded-md object-cover border bg-muted"
    />
  )
}
