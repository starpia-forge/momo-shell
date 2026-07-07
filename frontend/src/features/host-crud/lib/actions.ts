import { useHostStore } from '../../../entities/host'
import i18n from '../../../shared/i18n'
import type { Host } from '../../../shared/api/host'

// Native confirm() is enough here -- Webview2 supports it, and a bespoke
// confirm dialog would just be Dialog + two buttons for no real benefit.
export async function confirmAndDeleteHost(host: Host): Promise<boolean> {
  if (!window.confirm(i18n.t('hostForm.confirmDelete', { name: host.name }))) return false
  await useHostStore.getState().remove(host.id)
  return true
}
