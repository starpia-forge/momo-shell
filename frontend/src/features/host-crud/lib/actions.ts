import { useHostStore } from '../../../entities/host'
import type { Host } from '../../../shared/api/host'

// Native confirm() is enough here -- Webview2 supports it, and a bespoke
// confirm dialog would just be Dialog + two buttons for no real benefit.
export async function confirmAndDeleteHost(host: Host): Promise<boolean> {
  if (!window.confirm(`"${host.name}" 호스트를 삭제할까요?`)) return false
  await useHostStore.getState().remove(host.id)
  return true
}
