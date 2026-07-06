import { describe, expect, it } from 'vitest'
import { decodeSftpDrag, encodeSftpDrag, isSftpDrag, SFTP_MIME } from './dnd'

// vitest runs in a node environment (no DOM), so DataTransfer is mocked with
// just the surface these functions touch.
class FakeDataTransfer {
  private data: Record<string, string> = {}
  effectAllowed = ''

  setData(format: string, value: string) {
    this.data[format] = value
  }
  getData(format: string) {
    return this.data[format] ?? ''
  }
  get types() {
    return Object.keys(this.data)
  }
}

describe('encodeSftpDrag / decodeSftpDrag', () => {
  it('round-trips a payload', () => {
    const dt = new FakeDataTransfer() as unknown as DataTransfer
    encodeSftpDrag(dt, { side: 'local', paths: ['/a', '/b'] })
    expect(decodeSftpDrag(dt)).toEqual({ side: 'local', paths: ['/a', '/b'] })
  })

  it('decodes null for missing or garbage data', () => {
    const empty = new FakeDataTransfer() as unknown as DataTransfer
    expect(decodeSftpDrag(empty)).toBeNull()

    const garbage = new FakeDataTransfer() as unknown as DataTransfer
    garbage.setData(SFTP_MIME, '{not json')
    expect(decodeSftpDrag(garbage)).toBeNull()
  })
})

describe('isSftpDrag', () => {
  it('is true only when the sftp MIME is present without a Files type', () => {
    const dt = new FakeDataTransfer() as unknown as DataTransfer
    encodeSftpDrag(dt, { side: 'remote', paths: ['/x'] })
    expect(isSftpDrag(dt)).toBe(true)
  })

  it('excludes an OS file drag even if it somehow carries the sftp MIME', () => {
    const dt = new FakeDataTransfer()
    dt.setData(SFTP_MIME, '{}')
    dt.setData('Files', '')
    expect(isSftpDrag(dt as unknown as DataTransfer)).toBe(false)
  })

  it('is false for a plain OS file drag', () => {
    const dt = new FakeDataTransfer()
    dt.setData('Files', '')
    expect(isSftpDrag(dt as unknown as DataTransfer)).toBe(false)
  })
})
