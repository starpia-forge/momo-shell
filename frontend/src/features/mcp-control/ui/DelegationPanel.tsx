import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { killControl } from '../../../shared/api/mcp'
import { useHostStore } from '../../../entities/host'
import { useSessionStore } from '../../../entities/session'
import { Chip, StatusDot } from '../../../shared/ui'
import { registerDelegations } from '../lib/register'
import { useDelegationStore } from '../model/delegations'

/** Popover anchored under the tab bar's DelegationBadge, listing every
 * session currently under AI control with a per-session kill switch (US-1,
 * B5a), plus each session's cwd (OSC7-sourced, B5b), a live command-state
 * indicator (mcp:cmd-state, B5b), and any active fan-out scopes' "N/cap"
 * concurrent-session display (B5b). Anchored to the opposite corner from
 * TransferCenter (left instead of right) so the two floating popovers don't
 * overlap if both are open. */
export function DelegationPanel() {
  const { t } = useTranslation()
  const delegations = useDelegationStore((s) => s.delegations)
  const cmdStates = useDelegationStore((s) => s.cmdStates)
  const scopes = useDelegationStore((s) => s.scopes)
  const open = useDelegationStore((s) => s.open)
  const sessions = useSessionStore((s) => s.sessions)
  const hosts = useHostStore((s) => s.hosts)

  useEffect(() => registerDelegations(), [])

  const list = Object.values(delegations)
  if (!open || list.length === 0) return null

  return (
    <div className="fixed left-6 bottom-4 z-400 w-90 max-h-90 overflow-y-auto flex flex-col gap-1 p-2.5 rounded-xl bg-surface2 border border-line shadow-modal">
      {scopes.length > 0 && (
        <div className="flex flex-col gap-1 px-3 py-1.5 mb-1 border-b border-line">
          {scopes.map((scope) => (
            <div key={scope.scopeId} className="flex items-center justify-between gap-2 text-[11px] text-fg3">
              <span className="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{scope.hostNames.join(', ')}</span>
              <span className="flex-none font-mono">
                {t('delegation.scopeCap', { active: scope.activeCount, max: scope.maxConcurrent })}
              </span>
            </div>
          ))}
        </div>
      )}
      {list.map((deleg) => {
        const session = sessions[deleg.sessionId]
        const host = session?.hostId ? hosts[session.hostId] : undefined
        const label = host?.name ?? session?.shell ?? deleg.sessionId
        const cmdState = cmdStates[deleg.sessionId]
        return (
          <div key={deleg.sessionId} className="flex flex-col gap-1 px-3 py-2.5 rounded-md bg-inputbg">
            <div className="flex items-center gap-2">
              <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[12.5px] font-medium">
                {label}
              </span>
              {cmdState?.state === 'running' && <StatusDot status="connecting" />}
              {cmdState?.state === 'tui' && <Chip tone="neutral">{t('delegation.cmdTui')}</Chip>}
              {cmdState?.state === 'done' && (
                <Chip tone={cmdState.exitCode ? 'red' : 'green'}>
                  {t('delegation.cmdDone', { exitCode: cmdState.exitCode ?? 0 })}
                </Chip>
              )}
              <button
                className="flex-none border-none bg-transparent text-red cursor-pointer text-[12px] hover:opacity-80"
                onClick={() => void killControl(deleg.sessionId)}
                aria-label={t('delegation.kill')}
              >
                {t('delegation.kill')}
              </button>
            </div>
            {session?.cwd && (
              <span className="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[11px] font-mono text-fg3">
                {session.cwd}
              </span>
            )}
          </div>
        )
      })}
    </div>
  )
}
