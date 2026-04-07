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

export interface GenerateScriptRequest {
  filePaths: string[]
  outputDir: string
  trashDir: string
  scriptType: "bash" | "windows"
}

export interface GenerateScriptResponse {
  message: string
  scriptPath: string
  fileCount: number
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
