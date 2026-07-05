import { describe, expect, it } from 'vitest'
import { describeShareError } from './share'

describe('describeShareError', () => {
  it('recognizes the lockout message', () => {
    expect(describeShareError(new Error('shareclient: too many failed pairing attempts, try again later'))).toBe(
      '잠시 후 다시 시도하세요 (60초)'
    )
  })

  it('recognizes the cert-mismatch message', () => {
    expect(describeShareError(new Error('share: peer certificate fingerprint changed since pairing'))).toBe(
      '피어의 인증서가 변경되었습니다. 다시 페어링하세요'
    )
  })

  it('recognizes the unauthorized message', () => {
    expect(describeShareError(new Error('share: peer rejected our token'))).toBe('피어가 연결을 거부했습니다. 다시 페어링하세요')
  })

  it('falls back to a generic message for anything else', () => {
    expect(describeShareError(new Error('connection refused'))).toBe('요청이 실패했습니다')
    expect(describeShareError('not an Error object')).toBe('요청이 실패했습니다')
  })
})
