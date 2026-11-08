/**
 * 生图工作台网关调用：浏览器直接用用户自己的 API Key 调 /v1/images/*，
 * 与批量生图页（api/batchImage.ts）的鉴权方式一致。
 */
import { buildGatewayUrl } from './url'

export interface GatewayError extends Error {
  status: number
  code: string | number
  requestId: string
}

export async function parseGatewayError(response: Response): Promise<GatewayError> {
  let message = response.statusText || `HTTP ${response.status}`
  let code: string | number = response.status
  try {
    const body = await response.json()
    message = body?.error?.message || body?.message || message
    code = body?.error?.code || body?.error?.type || code
  } catch {
    // 非 JSON 错误体，保留状态文本
  }
  const error = new Error(message) as GatewayError
  error.status = response.status
  error.code = code
  error.requestId = response.headers.get('X-Request-Id') || ''
  return error
}

export interface ImageResultItem {
  b64_json?: string
  url?: string
  revised_prompt?: string
}

export interface ImageResultPayload {
  created?: number
  data?: ImageResultItem[]
}

export interface ImageTaskSubmitResponse {
  id: string
  task_id?: string
  status: string
  created_at: number
  expires_at: number
  poll_url?: string
}

export interface ImageTaskResponse {
  id: string
  status: 'processing' | 'completed' | 'failed'
  http_status?: number
  image_url?: string
  result?: ImageResultPayload
  error?: { error?: { message?: string; type?: string; code?: string } } & { message?: string }
  created_at: number
  completed_at?: number
  expires_at: number
}

export type ImageRequestBody = Record<string, unknown> | FormData

function endpointFor(mode: 'generate' | 'edit'): string {
  return mode === 'edit' ? '/v1/images/edits' : '/v1/images/generations'
}

function buildInit(apiKey: string, body: ImageRequestBody, signal?: AbortSignal): RequestInit {
  const headers: Record<string, string> = { Authorization: `Bearer ${apiKey}` }
  let payload: BodyInit
  if (body instanceof FormData) {
    payload = body
  } else {
    headers['Content-Type'] = 'application/json'
    payload = JSON.stringify(body)
  }
  return { method: 'POST', headers, body: payload, signal }
}

/** 异步任务未启用（未配置图片存储）时网关返回 404 */
export function isAsyncUnavailableError(error: unknown): boolean {
  const err = error as Partial<GatewayError> | undefined
  return err?.status === 404 && /async image tasks are not enabled/i.test(String(err?.message || ''))
}

export async function submitImageTask(
  apiKey: string,
  mode: 'generate' | 'edit',
  body: ImageRequestBody,
): Promise<ImageTaskSubmitResponse> {
  const response = await fetch(buildGatewayUrl(`${endpointFor(mode)}/async`), buildInit(apiKey, body))
  if (!response.ok) throw await parseGatewayError(response)
  return response.json()
}

export async function getImageTask(apiKey: string, taskId: string): Promise<ImageTaskResponse> {
  const response = await fetch(buildGatewayUrl(`/v1/images/tasks/${encodeURIComponent(taskId)}`), {
    headers: { Authorization: `Bearer ${apiKey}` },
    cache: 'no-store',
  })
  if (!response.ok) throw await parseGatewayError(response)
  return response.json()
}

export async function requestImagesSync(
  apiKey: string,
  mode: 'generate' | 'edit',
  body: ImageRequestBody,
  signal?: AbortSignal,
): Promise<ImageResultPayload> {
  const response = await fetch(buildGatewayUrl(endpointFor(mode)), buildInit(apiKey, body, signal))
  if (!response.ok) throw await parseGatewayError(response)
  return response.json()
}

export function taskErrorMessage(task: ImageTaskResponse): string {
  return task.error?.error?.message || task.error?.message || `HTTP ${task.http_status || 500}`
}
