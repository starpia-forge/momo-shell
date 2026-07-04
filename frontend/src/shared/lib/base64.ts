// Base64 <-> bytes helpers for the binary-safe terminal I/O transport.
//
// Decoded bytes must be fed to xterm.js as a Uint8Array, never as the raw
// atob() string -- atob() yields Latin-1 code points, and treating that as
// text (e.g. term.write(str)) mangles any multibyte UTF-8 output (Korean, etc).

type Base64Capable = { fromBase64?: (s: string) => Uint8Array }
type Base64Encodable = Uint8Array & { toBase64?: () => string }

export function b64ToBytes(b64: string): Uint8Array {
  const native = (Uint8Array as unknown as Base64Capable).fromBase64
  if (native) {
    return native(b64)
  }
  const binary = atob(b64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes
}

const CHUNK_SIZE = 0x8000

export function bytesToB64(bytes: Uint8Array): string {
  const native = (bytes as Base64Encodable).toBase64
  if (native) {
    return native.call(bytes)
  }
  let binary = ''
  for (let i = 0; i < bytes.length; i += CHUNK_SIZE) {
    binary += String.fromCharCode(...bytes.subarray(i, i + CHUNK_SIZE))
  }
  return btoa(binary)
}
