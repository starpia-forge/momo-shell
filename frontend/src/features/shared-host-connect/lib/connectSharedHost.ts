import { useHostStore } from '../../../entities/host'
import { openSSHSession, openSSHDirectSession } from '../../../entities/session'
import { setHostSecret, type AuthType, type Host } from '../../../shared/api/host'
import type { SharedHost } from '../../../shared/api/share'

export interface SharedHostCredentials {
  username: string
  authType: AuthType
  keyPath?: string
  secret: string
  /** "내 호스트로 저장하며 연결" -- persists a real Host row + secret, then
   * connects through the normal saved-host path instead of CreateSSHDirect. */
  saveAsHost: boolean
}

export interface SharedHostConnectResult {
  sessionId: string
  /** Present only when saveAsHost was true -- callers use this to route
   * through the same onConnect bridge as a regular saved-host connect. */
  host?: Host
}

export async function connectSharedHost(sharedHost: SharedHost, creds: SharedHostCredentials): Promise<SharedHostConnectResult> {
  if (creds.saveAsHost) {
    const host = await useHostStore.getState().save({
      name: sharedHost.name,
      address: sharedHost.address,
      port: sharedHost.port,
      labels: sharedHost.labels,
      username: creds.username,
      authType: creds.authType,
      keyPath: creds.keyPath,
    })
    if (creds.secret !== '') await setHostSecret(host.id, creds.secret)
    const sessionId = await openSSHSession(host.id, 80, 24)
    return { sessionId, host }
  }

  const sessionId = await openSSHDirectSession({
    name: sharedHost.name,
    address: sharedHost.address,
    port: sharedHost.port,
    username: creds.username,
    authType: creds.authType,
    keyPath: creds.keyPath,
    secret: creds.secret,
    cols: 80,
    rows: 24,
  })
  return { sessionId }
}
