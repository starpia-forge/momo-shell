import { useEffect, useMemo, useState, type ChangeEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Chip, SearchInput, Select } from '../../../shared/ui'
import { queryAudit, type AuditEvent } from '../../../shared/api/audit'
import { listMCPClients } from '../../../shared/api/mcp'
import { filterEvents } from '../lib/filterAudit'
import { formatRelativeTime } from '../lib/formatRelativeTime'
import { useAuditPanelStore } from '../model/store'
import { AuditReplayDialog } from './AuditReplayDialog'

const RISK_TONE: Record<string, 'green' | 'neutral' | 'red'> = { low: 'green', medium: 'neutral', high: 'red' }
const DECISION_TONE = (decision: string): 'green' | 'neutral' | 'red' =>
  decision === 'rejected' || decision === 'timeout' || decision === 'killed' ? 'red' : decision === 'auto' ? 'neutral' : 'green'

/** E5's audit panel (US-5): a post-hoc review timeline of every AI-control
 * decision (connect/command/control), filterable by client/decision/text.
 * Clicking a row with a session opens AuditReplayDialog for that session's
 * full id-ASC decision trail + captured output. Only mounted while RightDock
 * has the 'audit' tab active (mirrors HistoryPanel) -- mounting itself is
 * the signal to (re)load; US-5 is post-hoc review, not a live stream, so
 * there is no event subscription here. */
export function AuditPanel() {
  const { t } = useTranslation()
  const events = useAuditPanelStore((s) => s.events)
  const filterText = useAuditPanelStore((s) => s.filterText)
  const clientFilter = useAuditPanelStore((s) => s.clientFilter)
  const decisionFilter = useAuditPanelStore((s) => s.decisionFilter)
  const [clientNames, setClientNames] = useState<Record<string, string>>({})

  useEffect(() => {
    let cancelled = false
    void queryAudit('').then((result) => {
      if (!cancelled) useAuditPanelStore.getState().setEvents(result)
    })
    void listMCPClients().then((clients) => {
      if (!cancelled) setClientNames(Object.fromEntries(clients.map((c) => [c.id, c.name])))
    })
    return () => {
      cancelled = true
    }
  }, [])

  const clientOptions = useMemo(() => {
    const ids = Array.from(new Set(events.map((e) => e.clientId))).sort()
    return [{ value: '', label: t('auditPanel.filterAllClients') }, ...ids.map((id) => ({ value: id, label: clientNames[id] ?? id }))]
  }, [events, clientNames, t])

  const decisionOptions = useMemo(() => {
    const decisions = Array.from(new Set(events.map((e) => e.decision))).sort()
    return [{ value: '', label: t('auditPanel.filterAllDecisions') }, ...decisions.map((d) => ({ value: d, label: d }))]
  }, [events, t])

  const visible = filterEvents(events, { text: filterText, client: clientFilter, decision: decisionFilter })

  function handleRowClick(event: AuditEvent) {
    if (!event.sessionId) return
    useAuditPanelStore.getState().openReplay(event.sessionId)
  }

  return (
    <div className="flex flex-col flex-1 min-h-0 text-[13px] p-3.5 gap-3">
      <span className="text-[13.5px] font-bold">{t('auditPanel.title')}</span>
      <div className="flex gap-2">
        <Select
          className="flex-1"
          value={clientFilter}
          onChange={(v) => useAuditPanelStore.getState().setClientFilter(v)}
          options={clientOptions}
          aria-label={t('auditPanel.filterClient')}
        />
        <Select
          className="flex-1"
          value={decisionFilter}
          onChange={(v) => useAuditPanelStore.getState().setDecisionFilter(v)}
          options={decisionOptions}
          aria-label={t('auditPanel.filterDecision')}
        />
      </div>
      <SearchInput
        containerClassName="h-8"
        placeholder={t('auditPanel.filterPlaceholder')}
        value={filterText}
        onChange={(e: ChangeEvent<HTMLInputElement>) => useAuditPanelStore.getState().setFilterText(e.target.value)}
      />
      <div className="flex-1 -mx-3.5 overflow-y-auto">
        {visible.map((event) => (
          <div
            key={event.id}
            className="flex flex-col gap-1 mx-2.25 px-2.5 py-2.25 rounded-md cursor-pointer hover:bg-surface2"
            onClick={() => handleRowClick(event)}
          >
            <div className="flex items-center gap-1.5">
              <Chip>{t(`auditPanel.kind_${event.kind}`, { defaultValue: event.kind })}</Chip>
              {event.risk && <Chip tone={RISK_TONE[event.risk] ?? 'neutral'}>{event.risk}</Chip>}
              <Chip tone={DECISION_TONE(event.decision)}>{event.decision}</Chip>
              <span className="ml-auto flex-none text-[10.5px] text-fg3">{formatRelativeTime(event.timestamp)}</span>
            </div>
            <span
              className="overflow-hidden text-ellipsis whitespace-nowrap font-mono text-[12px]"
              title={event.originalCmd || event.target}
            >
              {event.originalCmd || event.target || event.sessionId}
            </span>
            <span className="text-[10.5px] text-fg3">{clientNames[event.clientId] ?? event.clientId}</span>
          </div>
        ))}
        {visible.length === 0 && <div className="px-2.5 py-2 text-[12px] text-fg3">{t('auditPanel.empty')}</div>}
      </div>
      <AuditReplayDialog />
    </div>
  )
}
