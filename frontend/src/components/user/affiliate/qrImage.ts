/**
 * 支付宝收款码处理：校验类型，并把大图压缩成 JPEG data URL，
 * 确保不超过后端 800KB（data URL 长度）的限制，同时保持二维码可识别。
 */

export const QR_ALLOWED_TYPES = ['image/png', 'image/jpeg', 'image/jpg', 'image/webp']
/** 与后端 AffiliateWithdrawQRImageMaxBytes 保持一致 */
export const QR_MAX_DATA_URL_LENGTH = 800 * 1024
const QR_MAX_EDGE = 1200

export type QRImageErrorCode = 'invalidType' | 'tooLarge' | 'readFailed'

export class QRImageError extends Error {
  constructor(public code: QRImageErrorCode) {
    super(code)
  }
}

function readAsDataURL(file: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(new QRImageError('readFailed'))
    reader.readAsDataURL(file)
  })
}

function loadImage(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => resolve(img)
    img.onerror = () => reject(new QRImageError('readFailed'))
    img.src = src
  })
}

/** 小图原样返回；超限时按最长边 1200px 缩放并以 JPEG 重新编码。 */
export async function prepareQRImage(file: File): Promise<string> {
  if (!QR_ALLOWED_TYPES.includes(file.type)) {
    throw new QRImageError('invalidType')
  }
  const original = await readAsDataURL(file)
  if (original.length <= QR_MAX_DATA_URL_LENGTH) {
    return original
  }

  const img = await loadImage(original)
  const scale = Math.min(1, QR_MAX_EDGE / Math.max(img.naturalWidth, img.naturalHeight))
  const width = Math.max(1, Math.round(img.naturalWidth * scale))
  const height = Math.max(1, Math.round(img.naturalHeight * scale))
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const ctx = canvas.getContext('2d')
  if (!ctx) {
    throw new QRImageError('readFailed')
  }
  // JPEG 不支持透明，先铺白底，避免透明区域变黑影响扫码
  ctx.fillStyle = '#ffffff'
  ctx.fillRect(0, 0, width, height)
  ctx.drawImage(img, 0, 0, width, height)

  for (const quality of [0.9, 0.8, 0.7]) {
    const out = canvas.toDataURL('image/jpeg', quality)
    if (out.length <= QR_MAX_DATA_URL_LENGTH) {
      return out
    }
  }
  throw new QRImageError('tooLarge')
}

/** 账号脱敏：保留前 3 位和后 2~4 位 */
export function maskAlipayAccount(account: string): string {
  const value = (account || '').trim()
  if (value.length <= 4) return value
  const at = value.indexOf('@')
  if (at > 0) {
    const name = value.slice(0, at)
    const keep = Math.min(3, Math.max(1, name.length - 1))
    return `${name.slice(0, keep)}***${value.slice(at)}`
  }
  const tail = value.length >= 8 ? 4 : 2
  return `${value.slice(0, 3)}****${value.slice(-tail)}`
}
