const encoder = new TextEncoder()
const decoder = new TextDecoder()

// バイト列の Base64URL 符号化
function bytesToBase64Url(bytes: Uint8Array): string {
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/u, '')
}

// Base64URL 文字列のバイト列復号
function base64UrlToBytes(value: string): Uint8Array {
  const normalized = value.replaceAll('-', '+').replaceAll('_', '/')
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=')
  const binary = atob(padded)
  return Uint8Array.from(binary, character => character.charCodeAt(0))
}

// 正規 Base64URL のみの厳密復号
function decodeCanonicalBase64Url(value: string): Uint8Array | null {
  if (!value || /[^A-Za-z0-9_-]/u.test(value)) return null
  try {
    const bytes = base64UrlToBytes(value)
    if (bytesToBase64Url(bytes) !== value) return null
    return bytes
  } catch {
    return null
  }
}

// Uint8Array から独立 ArrayBuffer への切出し
function exactBuffer(bytes: Uint8Array): ArrayBuffer {
  return bytes.slice().buffer as ArrayBuffer
}

// セッション秘密から AES-GCM 鍵の導出
async function encryptionKey(secret: string): Promise<CryptoKey> {
  if (secret.length < 32) throw new Error('NUXT_SESSION_SECRET must contain at least 32 characters')
  const digest = await crypto.subtle.digest('SHA-256', encoder.encode(secret))
  return crypto.subtle.importKey('raw', digest, 'AES-GCM', false, ['encrypt', 'decrypt'])
}

// JSON 値の AES-GCM 封印
export async function seal<T>(value: T, secret: string): Promise<string> {
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const key = await encryptionKey(secret)
  const plaintext = encoder.encode(JSON.stringify(value))
  const ciphertext = await crypto.subtle.encrypt({ name: 'AES-GCM', iv: exactBuffer(iv) }, key, exactBuffer(plaintext))
  return `v1.${bytesToBase64Url(iv)}.${bytesToBase64Url(new Uint8Array(ciphertext))}`
}

// 封印文字列の復号と JSON 復元
export async function unseal<T>(value: string, secret: string): Promise<T | null> {
  try {
    const [version, encodedIV, encodedCiphertext, extra] = value.split('.')
    if (version !== 'v1' || !encodedIV || !encodedCiphertext || extra) return null
    const iv = decodeCanonicalBase64Url(encodedIV)
    const ciphertext = decodeCanonicalBase64Url(encodedCiphertext)
    if (!iv || !ciphertext || iv.length !== 12) return null
    const key = await encryptionKey(secret)
    const plaintext = await crypto.subtle.decrypt(
      { name: 'AES-GCM', iv: exactBuffer(iv) },
      key,
      exactBuffer(ciphertext),
    )
    return JSON.parse(decoder.decode(plaintext)) as T
  } catch {
    return null
  }
}

// 乱数の Base64URL 生成
export function randomBase64Url(byteLength = 32): string {
  return bytesToBase64Url(crypto.getRandomValues(new Uint8Array(byteLength)))
}

// SHA-256 ダイジェストの Base64URL 化
export async function sha256Base64Url(value: string): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', encoder.encode(value))
  return bytesToBase64Url(new Uint8Array(digest))
}

// SHA-256 ダイジェストの16進化（Firebase の OIDC nonce 形式）
export async function sha256Hex(value: string): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', encoder.encode(value))
  return Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, '0')).join('')
}

// Base64URL JSON の復号
export function decodeBase64UrlJSON<T>(value: string): T {
  return JSON.parse(decoder.decode(base64UrlToBytes(value))) as T
}

// Base64URL のバイト列復号
export function decodeBase64Url(value: string): Uint8Array {
  return base64UrlToBytes(value)
}

// Base64URL の ArrayBuffer 復号
export function decodeBase64UrlBuffer(value: string): ArrayBuffer {
  return exactBuffer(base64UrlToBytes(value))
}

// 状態変更リクエストの同一オリジン判定（Cookie認証のCSRF対策。SameSite=Laxは同一サイトの別サブドメインを防げない）
export function isSameOriginWrite(method: string, headers: Headers, origin: string): boolean {
  if (['GET', 'HEAD', 'OPTIONS'].includes(method.toUpperCase())) return true
  const requestOrigin = headers.get('origin')
  if (requestOrigin) return requestOrigin === origin
  // Origin を送らない古い実装向け: Fetch Metadata で同一オリジンを確認
  return headers.get('sec-fetch-site') === 'same-origin'
}
