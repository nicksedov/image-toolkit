import { useState, useEffect, useCallback, useRef } from "react"

interface UseImageDimensionsReturn {
  imageRef: React.RefObject<HTMLImageElement | null>
  displayDimensions: { width: number; height: number } | null
  imageLoaded: boolean
  handleImageLoad: () => void
}

export function useImageDimensions(): UseImageDimensionsReturn {
  const [imageLoaded, setImageLoaded] = useState(false)
  const [displayDimensions, setDisplayDimensions] = useState<{ width: number; height: number } | null>(null)
  const imageRef = useRef<HTMLImageElement>(null)

  const handleImageLoad = useCallback(() => {
    if (imageRef.current) {
      const { clientWidth, clientHeight } = imageRef.current
      setDisplayDimensions({ width: clientWidth, height: clientHeight })
      setImageLoaded(true)
    }
  }, [])

  useEffect(() => {
    if (!imageLoaded || !imageRef.current) return

    const updateDimensions = () => {
      if (imageRef.current) {
        setDisplayDimensions({
          width: imageRef.current.clientWidth,
          height: imageRef.current.clientHeight,
        })
      }
    }

    updateDimensions()
    window.addEventListener("resize", updateDimensions)
    return () => window.removeEventListener("resize", updateDimensions)
  }, [imageLoaded])

  return { imageRef, displayDimensions, imageLoaded, handleImageLoad }
}
