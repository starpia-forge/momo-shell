import { openLocalSession, openSSHSession, useSessionStore } from '../../../entities/session'
import type { Host } from '../../../entities/host'
import { useTabStore } from '../model/store'

export async function createLocalTab(): Promise<void> {
  const sessionId = await openLocalSession({ cols: 80, rows: 24 })
  const shell = useSessionStore.getState().sessions[sessionId]?.shell ?? ''
  useTabStore.getState().addTab({
    id: sessionId,
    kind: 'local',
    sessionId,
    title: '로컬 쉘',
    subtitle: shell,
  })
}

export async function createSSHTab(host: Host): Promise<void> {
  const sessionId = await openSSHSession(host.id, 80, 24)
  useTabStore.getState().addTab({
    id: sessionId,
    kind: 'ssh',
    hostId: host.id,
    sessionId,
    title: host.name,
    subtitle: host.address,
  })
}
