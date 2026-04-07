export interface FileDTO {
  id: number
  path: string
  fileName: string
  dirPath: string
  modTime: string
}

export interface DuplicateGroupDTO {
  index: number
  hash: string
  size: number
  sizeHuman: string
  files: FileDTO[]
  thumbnail: string
}

export interface DuplicatesResponse {
  groups: DuplicateGroupDTO[]
  totalFiles: number
  pageFiles: number
  totalGroups: number
  scannedDirs: string[]
  currentPage: number
  pageSize: number
  totalPages: number
  hasPrevPage: boolean
  hasNextPage: boolean
  pageSizes: number[]
}

export interface ScanResponse {
  message: string
}

export interface ScanStatusResponse {
  scanning: boolean
  progress: string
  filesProcessed: number
}

export interface ThumbnailResponse {
  thumbnail: string
}

export interface DeleteFilesRequest {
  filePaths: string[]
  trashDir: string
}

export interface DeleteFilesResponse {
  success: number
  failed: number
  failedFiles?: string[]
}

export interface FolderPattern {
  id: string
  folders: string[]
  duplicateCount: number
  totalFiles: number
}

export interface FolderPatternsResponse {
  patterns: FolderPattern[]
}

export interface BatchDeleteRule {
  patternId: string
  keepFolder: string
}

export interface BatchDeleteRequest {
  rules: BatchDeleteRule[]
  trashDir: string
}

export interface BatchDeleteResponse {
  success: number
  failed: number
  failedFiles?: string[]
}

export interface ApiError {
  error: string
}

// --- Gallery Folder Types ---

export interface GalleryFolderDTO {
  id: number
  path: string
  fileCount: number
  createdAt: string
}

export interface GalleryFoldersResponse {
  folders: GalleryFolderDTO[]
  totalFolders: number
}

export interface AddFolderRequest {
  path: string
}

export interface AddFolderResponse {
  message: string
  folder: GalleryFolderDTO
  scanStarted: boolean
}

export interface RemoveFolderResponse {
  message: string
  filesRemoved: number
}

// --- Gallery Image Types ---

export interface GalleryImageDTO {
  id: number
  path: string
  fileName: string
  dirPath: string
  size: number
  sizeHuman: string
  modTime: string
  thumbnail?: string
}

export interface GalleryImagesResponse {
  images: GalleryImageDTO[]
  totalImages: number
  currentPage: number
  pageSize: number
  totalPages: number
  hasNextPage: boolean
}

// --- App Settings Types ---

export interface AppSettingsDTO {
  theme: "light" | "dark"
  language: "en" | "ru"
}

export interface UpdateSettingsRequest {
  theme?: "light" | "dark"
  language?: "en" | "ru"
}
