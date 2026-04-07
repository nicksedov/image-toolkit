import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatSize(size: number): string {
  const unit = 1024
  if (size < unit) return `${size} B`
  let div = unit
  let exp = 0
  for (let n = size / unit; n >= unit; n /= unit) {
    div *= unit
    exp++
  }
  return `${(size / div).toFixed(1)} ${"KMGTPE"[exp]}B`
}
