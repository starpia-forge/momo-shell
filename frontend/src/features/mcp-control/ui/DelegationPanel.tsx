import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { killControl } from '../../../shared/api/mcp'
import { useHostStore } from '../../../entities/host'
import { useSessionStore } from '../../../entities/session'
import { registerDelegations } from '../lib/register'
import { useDelegationStore } from '../model/delegations'

/** Popover anchored under the tab bar's DelegationBadge, listing every
 * session currently under AI control with a per-session kill switch (US-1,
 * B5a). Anchored to the opposite corner from TransferCenter (left instead
 * of right) so the two floating popovers don't overlap if both are open. */
export function DelegationPanel() {
  const { t } = useTranslation()
  const delegations = useDelegationStore((s) => s.delegations)
  const open = useDelegationStore((s) => s.open)
  const sessions = useSessionStore((s) => s.sessions)
  const hosts = useHostStore((s) => s.hosts)

  useEffect(() => registerDelegations(), [])

  const list = Object.values(delegations)
  if (!open || list.length === 0) return null

  return (
    <div className="fixed left-6 bottom-4 z-400 w-90 max-h-90 overflow-y-auto flex flex-col gap-1 p-2.5 rounded-xl bg-surface2 border border-line shadow-modal">
      {list.map((deleg) => {
        const session = sessions[deleg.sessionId]
        const host = session?.hostId ? hosts[session.hostId] : undefined
        const label = host?.name ?? session?.shell ?? deleg.sessionId
        return (
          <div key={deleg.sessionId} className="flex items-center gap-2 px-3 py-2.5 rounded-md bg-inputbg">
            <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[12.5px] font-medium">
              {label}
            </span>
            <button
              className="flex-none border-none bg-transparent text-red cursor-pointer text-[12px] hover:opacity-80"
              onClick={() => void killControl(deleg.sessionId)}
              aria-label={t('delegation.kill')}
            >
              {t('delegation.kill')}
            </button>
          </div>
        )
      })}
    </div>
  )
}
