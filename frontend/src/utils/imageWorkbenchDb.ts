/**
 * 生图工作台本地历史（IndexedDB）。
 * - records：每次生成的元数据（提示词、参数、状态、异步 task_id）
 * - blobs：原图与缩略图 Blob，单独存放，列表渲染只读缩略图
 * 数据按用户 ID 隔离（record.userId），同一浏览器多账号互不可见。
 */

import type { WorkbenchMode } from './imageWorkbench'

export type WorkbenchRecordStatus = 'pending' | 'completed' | 'failed' | 'expired'

export interface WorkbenchImage {
  blobId?: string
  thumbId?: string
  /** 无法转存为 Blob 时保留的远端链接（S3 预签名链接会过期） */
  url?: string
  revisedPrompt?: string
  width?: number
  height?: number
}

export interface WorkbenchRecordParams {
  size?: string
  quality?: string
  background?: string
  aspect_ratio?: string
  n: number
}

export interface WorkbenchRecord {
  id: string
  userId: number
  createdAt: number
  updatedAt: number
  mode: WorkbenchMode
  status: WorkbenchRecordStatus
  transport: 'async' | 'sync'
  taskId?: string
  taskExpiresAt?: number
  apiKeyId: number
  apiKeyName: string
  model: string
  prompt: string
  params: WorkbenchRecordParams
  referenceCount: number
  images: WorkbenchImage[]
  error?: string
  requestId?: string
  estimatedCost?: number | null
  /** 已通知过用户（避免重复弹 toast） */
  notified?: boolean
}

const DB_NAME = 'image-workbench'
const DB_VERSION = 1
const RECORDS = 'records'
const BLOBS = 'blobs'

let dbPromise: Promise<IDBDatabase> | null = null

function promisify<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
}

function txDone(tx: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
    tx.onabort = () => reject(tx.error)
  })
}

export function isIndexedDbAvailable(): boolean {
  return typeof indexedDB !== 'undefined'
}

function openDb(): Promise<IDBDatabase> {
  if (dbPromise) return dbPromise
  dbPromise = new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION)
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(RECORDS)) {
        const store = db.createObjectStore(RECORDS, { keyPath: 'id' })
        store.createIndex('userId', 'userId', { unique: false })
      }
      if (!db.objectStoreNames.contains(BLOBS)) {
        db.createObjectStore(BLOBS)
      }
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => {
      dbPromise = null
      reject(request.error)
    }
  })
  return dbPromise
}

export function newLocalId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}

export async function putRecord(record: WorkbenchRecord): Promise<void> {
  const db = await openDb()
  const tx = db.transaction(RECORDS, 'readwrite')
  tx.objectStore(RECORDS).put(JSON.parse(JSON.stringify(record)))
  await txDone(tx)
}

export async function getRecord(id: string): Promise<WorkbenchRecord | undefined> {
  const db = await openDb()
  return promisify(db.transaction(RECORDS).objectStore(RECORDS).get(id)) as Promise<WorkbenchRecord | undefined>
}

/** 按创建时间倒序返回该用户的全部记录（元数据很小，一次性读出即可）。 */
export async function listRecords(userId: number): Promise<WorkbenchRecord[]> {
  const db = await openDb()
  const index = db.transaction(RECORDS).objectStore(RECORDS).index('userId')
  const rows = (await promisify(index.getAll(IDBKeyRange.only(userId)))) as WorkbenchRecord[]
  return rows.sort((a, b) => b.createdAt - a.createdAt)
}

export async function putBlob(blob: Blob): Promise<string> {
  const id = newLocalId()
  const db = await openDb()
  const tx = db.transaction(BLOBS, 'readwrite')
  tx.objectStore(BLOBS).put(blob, id)
  await txDone(tx)
  return id
}

export async function getBlob(id: string): Promise<Blob | undefined> {
  const db = await openDb()
  return promisify(db.transaction(BLOBS).objectStore(BLOBS).get(id)) as Promise<Blob | undefined>
}

export async function deleteRecords(records: WorkbenchRecord[]): Promise<void> {
  if (records.length === 0) return
  const db = await openDb()
  const tx = db.transaction([RECORDS, BLOBS], 'readwrite')
  const recordStore = tx.objectStore(RECORDS)
  const blobStore = tx.objectStore(BLOBS)
  for (const record of records) {
    recordStore.delete(record.id)
    for (const image of record.images) {
      if (image.blobId) blobStore.delete(image.blobId)
      if (image.thumbId) blobStore.delete(image.thumbId)
    }
  }
  await txDone(tx)
}

export interface StorageUsage {
  usage: number
  quota: number
  persisted: boolean
}

export async function getStorageUsage(): Promise<StorageUsage | null> {
  if (typeof navigator === 'undefined' || !navigator.storage?.estimate) return null
  const estimate = await navigator.storage.estimate()
  const persisted = navigator.storage.persisted ? await navigator.storage.persisted() : false
  return { usage: estimate.usage ?? 0, quota: estimate.quota ?? 0, persisted }
}

/** 申请持久化存储，避免空间紧张时浏览器自动清理历史图片。 */
export async function requestPersistentStorage(): Promise<boolean> {
  if (typeof navigator === 'undefined' || !navigator.storage?.persist) return false
  try {
    if (navigator.storage.persisted && (await navigator.storage.persisted())) return true
    return await navigator.storage.persist()
  } catch {
    return false
  }
}

export function base64ToBlob(b64: string, mime = 'image/png'): Blob {
  const binary = atob(b64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i)
  return new Blob([bytes], { type: mime })
}

export function sniffImageMime(b64: string): string {
  if (b64.startsWith('/9j/')) return 'image/jpeg'
  if (b64.startsWith('UklGR')) return 'image/webp'
  return 'image/png'
}

/** 生成 256px 缩略图（WebP，不支持时回退 JPEG），失败时返回 null。 */
export async function createThumbnail(blob: Blob, maxEdge = 256): Promise<{ thumb: Blob | null; width: number; height: number }> {
  try {
    const bitmap = await createImageBitmap(blob)
    const { width, height } = bitmap
    const scale = Math.min(1, maxEdge / Math.max(width, height))
    const canvas = document.createElement('canvas')
    canvas.width = Math.max(1, Math.round(width * scale))
    canvas.height = Math.max(1, Math.round(height * scale))
    const ctx = canvas.getContext('2d')
    if (!ctx) return { thumb: null, width, height }
    ctx.drawImage(bitmap, 0, 0, canvas.width, canvas.height)
    bitmap.close?.()
    const thumb = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/webp', 0.82))
    return { thumb, width, height }
  } catch {
    return { thumb: null, width: 0, height: 0 }
  }
}

/** 下载一张历史图片（优先本地 Blob，否则打开远端链接）。 */
export async function downloadWorkbenchImage(image: WorkbenchImage, baseName: string) {
  const blob = image.blobId ? await getBlob(image.blobId) : undefined
  const href = blob ? URL.createObjectURL(blob) : image.url
  if (!href) return
  const ext = blob?.type.split('/')[1]?.replace('jpeg', 'jpg') || 'png'
  const anchor = document.createElement('a')
  anchor.href = href
  anchor.download = `${baseName}.${ext}`
  anchor.target = '_blank'
  anchor.rel = 'noopener'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  if (blob) setTimeout(() => URL.revokeObjectURL(href), 1000)
}
