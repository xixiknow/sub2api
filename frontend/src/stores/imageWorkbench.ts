/**
 * 生图工作台全局任务管理器。
 *
 * 任务不随页面组件卸载：
 * - 优先走异步任务接口（/v1/images/*\/async），task_id 立即写入 IndexedDB；
 *   用户切换页面或关闭标签页后，回到站点任意页面都会恢复轮询（任务在服务端保留 24 小时）。
 * - 站点未启用图片存储时异步接口返回 404，自动降级为同步请求；同步请求由本 store 持有，
 *   站内切换页面不会中断，但关闭标签页会中断。
 */
import { defineStore } from 'pinia'
import { computed, ref, shallowRef } from 'vue'
import { keysAPI } from '@/api/keys'
import {
  getImageTask,
  isAsyncUnavailableError,
  requestImagesSync,
  submitImageTask,
  taskErrorMessage,
  type GatewayError,
  type ImageRequestBody,
  type ImageResultPayload,
} from '@/api/imageWorkbench'
import {
  base64ToBlob,
  createThumbnail,
  deleteRecords,
  getRecord,
  isIndexedDbAvailable,
  listRecords,
  newLocalId,
  putBlob,
  putRecord,
  sniffImageMime,
  type WorkbenchImage,
  type WorkbenchRecord,
  type WorkbenchRecordParams,
} from '@/utils/imageWorkbenchDb'
import type { WorkbenchMode } from '@/utils/imageWorkbench'
import type { ApiKey } from '@/types'
import { i18n } from '@/i18n'
import { useAppStore } from './app'

export interface WorkbenchSubmitInput {
  apiKey: ApiKey
  mode: WorkbenchMode
  model: string
  prompt: string
  params: WorkbenchRecordParams
  references: File[]
  mask?: File | null
  estimatedCost?: number | null
}

const POLL_FAST_MS = 3000
const POLL_MEDIUM_MS = 6000
const POLL_SLOW_MS = 12000
const POLL_HIDDEN_MS = 20000

function t(key: string, params?: Record<string, unknown>): string {
  const translate = i18n.global.t as unknown as (k: string, p?: Record<string, unknown>) => string
  return params ? translate(key, params) : translate(key)
}

export function describeGatewayError(error: unknown): string {
  const err = error as Partial<GatewayError> | undefined
  const status = err?.status
  const base = err?.message || String(error)
  let friendly = ''
  if (status === 401) friendly = t('imageWorkbench.errors.unauthorized')
  else if (status === 402) friendly = t('imageWorkbench.errors.insufficientBalance')
  else if (status === 403) friendly = t('imageWorkbench.errors.forbidden')
  else if (status === 429) friendly = t('imageWorkbench.errors.rateLimited')
  else if (err instanceof TypeError) friendly = t('imageWorkbench.errors.network')
  return friendly ? `${friendly}（${base}）` : base
}

function buildRequestBody(input: WorkbenchSubmitInput): ImageRequestBody {
  const fields: Record<string, string | number> = {
    model: input.model,
    prompt: input.prompt,
    n: input.params.n,
  }
  if (input.params.size && input.params.size !== 'auto') fields.size = input.params.size
  if (input.params.quality && input.params.quality !== 'auto') fields.quality = input.params.quality
  if (input.params.background && input.params.background !== 'auto') fields.background = input.params.background
  if (input.params.aspect_ratio) fields.aspect_ratio = input.params.aspect_ratio

  if (input.mode === 'generate') return fields

  const form = new FormData()
  for (const [key, value] of Object.entries(fields)) form.append(key, String(value))
  const fieldName = input.references.length > 1 ? 'image[]' : 'image'
  for (const file of input.references) form.append(fieldName, file, file.name)
  if (input.mask) form.append('mask', input.mask, input.mask.name)
  return form
}

async function fetchRemoteImage(url: string): Promise<Blob | null> {
  try {
    const response = await fetch(url, { mode: 'cors', cache: 'no-store' })
    if (!response.ok) return null
    const blob = await response.blob()
    return blob.type.startsWith('image/') || blob.size > 0 ? blob : null
  } catch {
    return null
  }
}

async function persistResultImages(payload: ImageResultPayload | undefined, fallbackUrl?: string): Promise<WorkbenchImage[]> {
  const items = payload?.data?.length ? payload.data : fallbackUrl ? [{ url: fallbackUrl }] : []
  const images: WorkbenchImage[] = []
  for (const item of items) {
    let blob: Blob | null = null
    if (item.b64_json) blob = base64ToBlob(item.b64_json, sniffImageMime(item.b64_json))
    else if (item.url) blob = await fetchRemoteImage(item.url)

    if (!blob) {
      if (item.url) images.push({ url: item.url, revisedPrompt: item.revised_prompt })
      continue
    }
    const { thumb, width, height } = await createThumbnail(blob)
    const blobId = await putBlob(blob)
    const thumbId = thumb ? await putBlob(thumb) : undefined
    images.push({ blobId, thumbId, width, height, revisedPrompt: item.revised_prompt })
  }
  return images
}

export const useImageWorkbenchStore = defineStore('imageWorkbench', () => {
  const records = ref<WorkbenchRecord[]>([])
  const userId = ref<number | null>(null)
  const loaded = ref(false)
  /** null = 尚未探测；false = 本站未启用异步任务（未配置图片存储） */
  const asyncSupported = ref<boolean | null>(null)
  /** 工作台页面是否处于打开状态（打开时不弹完成 toast，结果直接展示在页面上） */
  const viewActive = ref(false)

  const syncControllers = shallowRef(new Map<string, AbortController>())
  const keySecrets = new Map<number, string>()
  let pollTimer: ReturnType<typeof setTimeout> | null = null
  let polling = false
  let initPromise: Promise<void> | null = null

  const pendingRecords = computed(() => records.value.filter((record) => record.status === 'pending'))
  const pendingAsync = computed(() => pendingRecords.value.filter((record) => record.transport === 'async'))

  function upsertLocal(record: WorkbenchRecord) {
    const index = records.value.findIndex((item) => item.id === record.id)
    if (index >= 0) records.value.splice(index, 1, record)
    else records.value.unshift(record)
  }

  async function saveRecord(record: WorkbenchRecord) {
    record.updatedAt = Date.now()
    upsertLocal({ ...record })
    await putRecord(record)
  }

  async function init(currentUserId: number) {
    if (!isIndexedDbAvailable()) return
    if (userId.value === currentUserId && (loaded.value || initPromise)) return initPromise ?? undefined
    reset()
    userId.value = currentUserId
    initPromise = (async () => {
      const rows = await listRecords(currentUserId)
      const now = Date.now()
      for (const row of rows) {
        if (row.status !== 'pending') continue
        if (row.transport === 'sync') {
          // 同步请求只活在上一个标签页里，页面关闭后无法恢复
          row.status = 'failed'
          row.error = t('imageWorkbench.errors.syncInterrupted')
          await putRecord(row)
        } else if (row.taskExpiresAt && row.taskExpiresAt * 1000 < now) {
          row.status = 'expired'
          row.error = t('imageWorkbench.errors.taskExpired')
          await putRecord(row)
        }
      }
      records.value = rows
      loaded.value = true
      schedulePoll(0)
    })()
    try {
      await initPromise
    } finally {
      initPromise = null
    }
  }

  function reset() {
    if (pollTimer) clearTimeout(pollTimer)
    pollTimer = null
    for (const controller of syncControllers.value.values()) controller.abort()
    syncControllers.value = new Map()
    keySecrets.clear()
    records.value = []
    loaded.value = false
    userId.value = null
  }

  async function resolveKeySecret(apiKeyId: number): Promise<string | null> {
    const cached = keySecrets.get(apiKeyId)
    if (cached) return cached
    try {
      const key = await keysAPI.getById(apiKeyId)
      if (key?.key) {
        keySecrets.set(apiKeyId, key.key)
        return key.key
      }
    } catch {
      // Key 已删除或无权限
    }
    return null
  }

  function nextPollDelay(): number {
    if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return POLL_HIDDEN_MS
    const youngest = Math.max(...pendingAsync.value.map((record) => record.createdAt))
    const age = Date.now() - youngest
    if (age < 60_000) return POLL_FAST_MS
    if (age < 180_000) return POLL_MEDIUM_MS
    return POLL_SLOW_MS
  }

  function schedulePoll(delay?: number) {
    if (pollTimer) clearTimeout(pollTimer)
    pollTimer = null
    if (pendingAsync.value.length === 0) return
    pollTimer = setTimeout(() => void pollOnce(), delay ?? nextPollDelay())
  }

  async function pollOnce() {
    if (polling) return
    polling = true
    try {
      for (const record of [...pendingAsync.value]) {
        if (!record.taskId) continue
        const secret = await resolveKeySecret(record.apiKeyId)
        if (!secret) {
          await finishFailed(record, t('imageWorkbench.errors.keyUnavailable'))
          continue
        }
        try {
          const task = await getImageTask(secret, record.taskId)
          if (task.status === 'completed') {
            const images = await persistResultImages(task.result, task.image_url)
            await finishCompleted(record, images)
          } else if (task.status === 'failed') {
            await finishFailed(record, taskErrorMessage(task))
          }
        } catch (error) {
          const status = (error as Partial<GatewayError>)?.status
          if (status === 404) {
            await finishFailed(record, t('imageWorkbench.errors.taskExpired'), 'expired')
          } else if (status === 401 || status === 403) {
            await finishFailed(record, describeGatewayError(error))
          }
          // 其他错误（网络抖动、5xx）保留 pending，下一轮重试
        }
      }
    } finally {
      polling = false
      schedulePoll()
    }
  }

  async function finishCompleted(record: WorkbenchRecord, images: WorkbenchImage[]) {
    const latest = (await getRecord(record.id)) ?? record
    if (latest.status !== 'pending') return
    latest.status = images.length > 0 ? 'completed' : 'failed'
    latest.images = images
    if (images.length === 0) latest.error = t('imageWorkbench.errors.emptyResult')
    await notify(latest)
    await saveRecord(latest)
  }

  async function finishFailed(
    record: WorkbenchRecord,
    message: string,
    status: 'failed' | 'expired' = 'failed',
    requestId?: string,
  ) {
    const latest = (await getRecord(record.id)) ?? record
    if (latest.status !== 'pending') return
    latest.status = status
    latest.error = message
    if (requestId) latest.requestId = requestId
    await notify(latest)
    await saveRecord(latest)
  }

  async function notify(record: WorkbenchRecord) {
    if (record.notified) return
    record.notified = true
    if (viewActive.value) return
    const appStore = useAppStore()
    if (record.status === 'completed') appStore.showSuccess(t('imageWorkbench.toast.completed'), 6000)
    else appStore.showError(t('imageWorkbench.toast.failed', { error: record.error || '' }), 8000)
  }

  async function submit(input: WorkbenchSubmitInput): Promise<WorkbenchRecord> {
    if (userId.value === null) throw new Error('image workbench is not initialized')
    keySecrets.set(input.apiKey.id, input.apiKey.key)
    const now = Date.now()
    const record: WorkbenchRecord = {
      id: newLocalId(),
      userId: userId.value,
      createdAt: now,
      updatedAt: now,
      mode: input.mode,
      status: 'pending',
      transport: 'async',
      apiKeyId: input.apiKey.id,
      apiKeyName: input.apiKey.name,
      model: input.model,
      prompt: input.prompt,
      params: { ...input.params },
      referenceCount: input.references.length,
      images: [],
      estimatedCost: input.estimatedCost ?? null,
    }
    const body = buildRequestBody(input)

    if (asyncSupported.value !== false) {
      try {
        const task = await submitImageTask(input.apiKey.key, input.mode, body)
        asyncSupported.value = true
        record.taskId = task.id
        record.taskExpiresAt = task.expires_at
        await saveRecord(record)
        schedulePoll(POLL_FAST_MS)
        return record
      } catch (error) {
        if (!isAsyncUnavailableError(error)) throw error
        asyncSupported.value = false
      }
    }

    record.transport = 'sync'
    await saveRecord(record)
    void runSync(record, input.apiKey.key, input.mode, body)
    return record
  }

  async function runSync(record: WorkbenchRecord, secret: string, mode: WorkbenchMode, body: ImageRequestBody) {
    const controller = new AbortController()
    const controllers = new Map(syncControllers.value)
    controllers.set(record.id, controller)
    syncControllers.value = controllers
    try {
      const payload = await requestImagesSync(secret, mode, body, controller.signal)
      const images = await persistResultImages(payload)
      await finishCompleted(record, images)
    } catch (error) {
      if (controller.signal.aborted) {
        await finishFailed(record, t('imageWorkbench.errors.cancelled'))
      } else {
        await finishFailed(record, describeGatewayError(error), 'failed', (error as Partial<GatewayError>)?.requestId)
      }
    } finally {
      const next = new Map(syncControllers.value)
      next.delete(record.id)
      syncControllers.value = next
    }
  }

  function canCancel(record: WorkbenchRecord): boolean {
    return record.status === 'pending' && syncControllers.value.has(record.id)
  }

  function cancel(record: WorkbenchRecord) {
    syncControllers.value.get(record.id)?.abort()
  }

  async function remove(ids: string[]) {
    const targets = records.value.filter((record) => ids.includes(record.id))
    for (const record of targets) syncControllers.value.get(record.id)?.abort()
    await deleteRecords(targets)
    records.value = records.value.filter((record) => !ids.includes(record.id))
  }

  /** 清理最旧的 N 条已结束记录（进行中的任务不动） */
  async function removeOldest(count: number) {
    const finished = records.value.filter((record) => record.status !== 'pending')
    const oldest = finished.slice(-count).map((record) => record.id)
    await remove(oldest)
  }

  const imageCount = computed(() => records.value.reduce((sum, record) => sum + record.images.length, 0))

  return {
    records,
    loaded,
    asyncSupported,
    viewActive,
    pendingRecords,
    imageCount,
    init,
    reset,
    submit,
    cancel,
    canCancel,
    remove,
    removeOldest,
    pollNow: () => schedulePoll(0),
  }
})
