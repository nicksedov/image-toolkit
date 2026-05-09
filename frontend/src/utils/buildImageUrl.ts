export function buildImageUrl(imagePath: string, apiEndpoint: string, params?: Record<string, string | number>) {
  const url = new URL(apiEndpoint, import.meta.env.VITE_API_URL || "")
  url.searchParams.set("path", imagePath)
  if (params) {
    Object.entries(params).forEach(([key, value]) => {
      url.searchParams.set(key, String(value))
    })
  }
  return url.toString()
}
