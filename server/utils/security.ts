const encoder = new TextEncoder()
const decoder = new TextDecoder()

function bytesToBase64Url(bytes: Uint8Array): string {
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/u, '')
}

function base64UrlToBytes(value: string): Uint8Array {
  const normalized = value.replaceAll('-', '+').replaceAll('_', '/')
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=')
  const binary = atob(padded)
  return Uint8Array.from(binary, character => character.charCodeAt(0))
}

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

function exactBuffer(bytes: Uint8Array): ArrayBuffer {
  return bytes.slice().buffer as ArrayBuffer
}

async function encryptionKey(secret: string): Promise<CryptoKey> {
  if (secret.length < 32) throw new Error('NUXT_SESSION_SECRET must contain at least 32 characters')
  const digest = await crypto.subtle.digest('SHA-256', encoder.encode(secret))
  return crypto.subtle.importKey('raw', digest, 'AES-GCM', false, ['encrypt', 'decrypt'])
}

export async function seal<T>(value: T, secret: string): Promise<string> {
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const key = await encryptionKey(secret)
  const plaintext = encoder.encode(JSON.stringify(value))
  const ciphertext = await crypto.subtle.encrypt({ name: 'AES-GCM', iv: exactBuffer(iv) }, key, exactBuffer(plaintext))
  return `v1.${bytesToBase64Url(iv)}.${bytesToBase64Url(new Uint8Array(ciphertext))}`
}

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

export function randomBase64Url(byteLength = 32): string {
  return bytesToBase64Url(crypto.getRandomValues(new Uint8Array(byteLength)))
}

export async function sha256Base64Url(value: string): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', encoder.encode(value))
  return bytesToBase64Url(new Uint8Array(digest))
}

export function decodeBase64UrlJSON<T>(value: string): T {
  return JSON.parse(decoder.decode(base64UrlToBytes(value))) as T
}

export function decodeBase64Url(value: string): Uint8Array {
  return base64UrlToBytes(value)
}

export function decodeBase64UrlBuffer(value: string): ArrayBuffer {
  return exactBuffer(base64UrlToBytes(value))
}
