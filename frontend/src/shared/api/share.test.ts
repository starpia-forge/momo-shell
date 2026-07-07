import { beforeAll, describe, expect, it } from 'vitest'
import { describeShareError } from './share'
import i18n from '../i18n'

beforeAll(async () => {
  await i18n.changeLanguage('en')
})

describe('describeShareError', () => {
  it('recognizes the lockout message', () => {
    expect(describeShareError(new Error('shareclient: too many failed pairing attempts, try again later'))).toBe(
      'Try again in a moment (60s)'
    )
  })

  it('recognizes the cert-mismatch message', () => {
    expect(describeShareError(new Error('share: peer certificate fingerprint changed since pairing'))).toBe(
      "The peer's certificate has changed. Please pair again"
    )
  })

  it('recognizes the unauthorized message', () => {
    expect(describeShareError(new Error('share: peer rejected our token'))).toBe('The peer rejected the connection. Please pair again')
  })

  it('falls back to a generic message for anything else', () => {
    expect(describeShareError(new Error('connection refused'))).toBe('Request failed')
    expect(describeShareError('not an Error object')).toBe('Request failed')
  })
})
