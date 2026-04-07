import { apiGet, apiPost, apiDelete, apiPut } from "./client"
import type {
  DuplicatesResponse,
  ScanResponse,
  ScanStatusResponse,
  ThumbnailResponse,
  GenerateScriptRequest,
  GenerateScriptResponse,
  DeleteFilesRequest,
  DeleteFilesResponse,
  FolderPatternsResponse,
  BatchDeleteRequest,
  BatchDeleteResponse,
  GalleryFoldersResponse,
  AddFolderRequest,
  AddFolderResponse,
  RemoveFolderResponse,
  GalleryImagesResponse,
  AppSettingsDTO,
  UpdateSettingsRequest,
} from "@/types"

export function fetchDuplicates(page: number, pageSize: number): Promise<DuplicatesResponse> {
  return apiGet<DuplicatesResponse>("/api/duplicates", {
    page: String(page),
    pageSize: String(pageSize),
  })
}

export function triggerScan(): Promise<ScanResponse> {
  return apiPost<ScanResponse>("/api/scan")
}

export function fetchScanStatus(): Promise<ScanStatusResponse> {
  return apiGet<ScanStatusResponse>("/api/status")
}

export function fetchThumbnail(path: string): Promise<ThumbnailResponse> {
  return apiGet<ThumbnailResponse>("/api/thumbnail", { path })
}

export function generateScript(req: GenerateScriptRequest): Promise<GenerateScriptResponse> {
  return apiPost<GenerateScriptResponse>("/api/generate-script", req)
}

export function deleteFiles(req: DeleteFilesRequest): Promise<DeleteFilesResponse> {
  return apiPost<DeleteFilesResponse>("/api/delete-files", req)
}

export function fetchFolderPatterns(): Promise<FolderPatternsResponse> {
  return apiGet<FolderPatternsResponse>("/api/folder-patterns")
}

export function batchDelete(req: BatchDeleteRequest): Promise<BatchDeleteResponse> {
  return apiPost<BatchDeleteResponse>("/api/batch-delete", req)
}

// --- Gallery Folders ---

export function fetchFolders(): Promise<GalleryFoldersResponse> {
  return apiGet<GalleryFoldersResponse>("/api/folders")
}

export function addFolder(req: AddFolderRequest): Promise<AddFolderResponse> {
  return apiPost<AddFolderResponse>("/api/folders", req)
}

export function removeFolder(id: number): Promise<RemoveFolderResponse> {
  return apiDelete<RemoveFolderResponse>(`/api/folders/${id}`)
}

// --- Gallery Images ---

export function fetchGalleryImages(
  page: number,
  pageSize: number,
  view: string
): Promise<GalleryImagesResponse> {
  return apiGet<GalleryImagesResponse>("/api/gallery", {
    page: String(page),
    pageSize: String(pageSize),
    view,
  })
}

// --- App Settings ---

export function fetchSettings(): Promise<AppSettingsDTO> {
  return apiGet<AppSettingsDTO>("/api/settings")
}

export function updateSettings(req: UpdateSettingsRequest): Promise<AppSettingsDTO> {
  return apiPut<AppSettingsDTO>("/api/settings", req)
}
