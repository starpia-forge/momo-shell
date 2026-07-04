import { openSSHSession } from '../../../entities/session'

// A thin wrapper today (registry + default size); once widgets/tab-bar
// exists (next step) this is where a new Tab gets created alongside the
// session.
export async function connectHost(hostId: string): Promise<string> {
  return openSSHSession(hostId, 80, 24)
}
