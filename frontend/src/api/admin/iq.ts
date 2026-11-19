/**
 * 模型智力测试 API（fork 扩展）
 */
import { apiClient } from '../client'

export type IQVerdict = 'passed' | 'failed' | 'inconclusive' | 'error'
export type IQFailAction = 'observe_only' | 'health_penalty' | 'disable_scheduling'

export interface IQQuestion {
  id: number
  slug: string
  kind: 'candy' | 'random_reasoning' | 'svg_draw'
  title: string
  prompt: string
  keywords: string[]
  builtin: boolean
  enabled: boolean
}

export interface IQSampleStat {
  question_slug: string
  model_id: string
  label: string
  count: number
}

export interface IQQuestionsResponse {
  questions: IQQuestion[]
  sample_stats: IQSampleStat[] | null
  min_reference_samples: number
  recommended_samples: number
  min_classifier_samples: number
}

export interface IQSVGScore {
  distance: number
  pass_threshold: number
  fail_threshold: number
  style_similarity: number
  style_threshold: number
  reference_scope: string
  reference_count: number
  worst_features?: Record<string, number>
  classifier_prob?: number
  warnings?: string[]
}

export interface IQTestResult {
  id: number
  plan_id?: number
  account_id: number
  model_id: string
  question_slug: string
  round_id: string
  prompt: string
  response_text: string
  verdict: IQVerdict
  score?: number
  detail: {
    reason?: string
    hard_fails?: string[]
    svg_score?: IQSVGScore
    random_family?: string
    reference?: string
    anim_source?: string
    [key: string]: unknown
  }
  error_message: string
  label: string
  latency_ms: number
  created_at: string
}

export interface IQRoundResult {
  round_id: string
  verdict: IQVerdict
  results: IQTestResult[]
}

export interface IQTestPlan {
  id: number
  account_id: number
  model_id: string
  question_slug: string
  parallel_count: number
  cron_expression: string
  enabled: boolean
  fail_action: IQFailAction
  fail_threshold: number
  consecutive_fails: number
  action_applied: boolean
  max_results: number
  last_verdict: string
  last_run_at: string | null
  next_run_at: string | null
}

export interface IQPlanPayload {
  model_id: string
  question_slug: string
  parallel_count: number
  cron_expression: string
  enabled: boolean
  fail_action: IQFailAction
  fail_threshold: number
  max_results?: number
}

export interface IQReferenceSample {
  id: number
  question_slug: string
  model_id: string
  label: 'good' | 'bad'
  source: string
  note: string
  created_at: string
}

export interface IQClassifierInfo {
  question_slug: string
  n_good: number
  n_bad: number
  accuracy: number
  trained_at: string
}

export interface IQUploadResult {
  created: IQReferenceSample[] | null
  failed: { file?: string; error: string }[] | null
}

const LONG_TIMEOUT = 10 * 60 * 1000

export async function listQuestions(): Promise<IQQuestionsResponse> {
  const { data } = await apiClient.get<IQQuestionsResponse>('/admin/iq/questions')
  return data
}

export async function saveQuestion(payload: { slug: string; title: string; prompt: string; keywords: string[] }): Promise<IQQuestion> {
  const { data } = await apiClient.post<IQQuestion>('/admin/iq/questions', payload)
  return data
}

export async function deleteQuestion(slug: string): Promise<void> {
  await apiClient.delete(`/admin/iq/questions/${encodeURIComponent(slug)}`)
}

export async function runAccountTest(accountId: number, payload: { model_id: string; question_slug: string; parallel_count: number }): Promise<IQRoundResult> {
  const { data } = await apiClient.post<IQRoundResult>(`/admin/accounts/${accountId}/iq-test`, payload, { timeout: LONG_TIMEOUT })
  return data
}

export async function listResults(params: { account_id?: number; plan_id?: number; limit?: number }): Promise<IQTestResult[]> {
  const { data } = await apiClient.get<IQTestResult[] | null>('/admin/iq/results', { params })
  return data ?? []
}

export async function labelResult(id: number, label: 'good' | 'bad'): Promise<void> {
  await apiClient.post(`/admin/iq/results/${id}/label`, { label })
}

export async function listPlans(accountId: number): Promise<IQTestPlan[]> {
  const { data } = await apiClient.get<IQTestPlan[] | null>(`/admin/accounts/${accountId}/iq-plans`)
  return data ?? []
}

export async function createPlan(accountId: number, payload: IQPlanPayload): Promise<IQTestPlan> {
  const { data } = await apiClient.post<IQTestPlan>(`/admin/accounts/${accountId}/iq-plans`, payload)
  return data
}

export async function updatePlan(id: number, payload: IQPlanPayload): Promise<IQTestPlan> {
  const { data } = await apiClient.put<IQTestPlan>(`/admin/iq/plans/${id}`, payload)
  return data
}

export async function deletePlan(id: number): Promise<void> {
  await apiClient.delete(`/admin/iq/plans/${id}`)
}

export async function runPlan(id: number): Promise<IQRoundResult> {
  const { data } = await apiClient.post<IQRoundResult>(`/admin/iq/plans/${id}/run`, {}, { timeout: LONG_TIMEOUT })
  return data
}

export async function listSamples(slug: string): Promise<{ samples: IQReferenceSample[] | null; classifier: IQClassifierInfo | null }> {
  const { data } = await apiClient.get<{ samples: IQReferenceSample[] | null; classifier: IQClassifierInfo | null }>(
    `/admin/iq/questions/${encodeURIComponent(slug)}/samples`
  )
  return data
}

export async function uploadSampleFiles(slug: string, files: File[], modelId: string, label: 'good' | 'bad'): Promise<IQUploadResult> {
  const form = new FormData()
  files.forEach((f) => form.append('files', f))
  form.append('model_id', modelId)
  form.append('label', label)
  const { data } = await apiClient.post<IQUploadResult>(`/admin/iq/questions/${encodeURIComponent(slug)}/samples`, form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: LONG_TIMEOUT
  })
  return data
}

export async function uploadSampleText(slug: string, content: string, modelId: string, label: 'good' | 'bad', note = ''): Promise<IQUploadResult> {
  const { data } = await apiClient.post<IQUploadResult>(`/admin/iq/questions/${encodeURIComponent(slug)}/samples`, {
    content,
    model_id: modelId,
    label,
    note
  })
  return data
}

export async function deleteSample(slug: string, id: number): Promise<void> {
  await apiClient.delete(`/admin/iq/questions/${encodeURIComponent(slug)}/samples/${id}`)
}

export async function retrainClassifier(slug: string): Promise<IQClassifierInfo> {
  const { data } = await apiClient.post<IQClassifierInfo>(`/admin/iq/questions/${encodeURIComponent(slug)}/classifier/retrain`, {})
  return data
}

export const iqAPI = {
  listQuestions,
  saveQuestion,
  deleteQuestion,
  runAccountTest,
  listResults,
  labelResult,
  listPlans,
  createPlan,
  updatePlan,
  deletePlan,
  runPlan,
  listSamples,
  uploadSampleFiles,
  uploadSampleText,
  deleteSample,
  retrainClassifier
}

export default iqAPI
