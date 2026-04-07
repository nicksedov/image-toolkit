import { apiGet, apiPost } from "./client"
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
