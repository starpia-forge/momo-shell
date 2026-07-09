import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Dialog, Button, Chip } from '../../../shared/ui'
import {
  respondCommandApproval,
  respondConnectApproval,
  respondConnectionScopeApproval,
  respondControlApproval,
} from '../../../shared/api/mcp'
import {
  subscribe,
  topics,
  type MCPCommandApprovalPayload,
  type MCPConnectApprovalPayload,
  type MCPConnectionScopeApprovalPayload,
  type MCPControlApprovalPayload,
} from '../../../shared/api/events'
import { scheduleApprovalTTL, useApprovalQueueStore, type ApprovalRequest } from '../model/approvalQueue'

// Mounted once (see pages/workspace) -- same one-at-a-time shape as
// MCPPairApprovalDialog, covering the other 4 mcp:*-approval kinds in a
// single dialog since they share the exact same subscribe/store/respond
// skeleton and only ever need to show one at a time.
export function MCPApprovalDialog() {
  const { t } = useTranslation()

  useEffect(() => {
    const unsubs = [
      subscribe<MCPConnectApprovalPayload>(topics.mcpConnectApproval(), (payload) => {
        useApprovalQueueStore.getState().setRequest(payload.requestId, { kind: 'connect', payload })
        scheduleApprovalTTL(payload.requestId)
      }),
      subscribe<MCPControlApprovalPayload>(topics.mcpControlApproval(), (payload) => {
        useApprovalQueueStore.getState().setRequest(payload.requestId, { kind: 'control', payload })
        scheduleApprovalTTL(payload.requestId)
      }),
      subscribe<MCPConnectionScopeApprovalPayload>(topics.mcpConnectionScopeApproval(), (payload) => {
        useApprovalQueueStore.getState().setRequest(payload.requestId, { kind: 'scope', payload })
        scheduleApprovalTTL(payload.requestId)
      }),
      subscribe<MCPCommandApprovalPayload>(topics.mcpCommandApproval(), (payload) => {
        useApprovalQueueStore.getState().setRequest(payload.requestId, { kind: 'command', payload })
        scheduleApprovalTTL(payload.requestId)
      }),
    ]
    return () => unsubs.forEach((unsub) => unsub())
  }, [])

  const requests = useApprovalQueueStore((s) => s.requests)
  const entry = Object.entries(requests)[0]
  if (!entry) return null
  const [requestId, request] = entry

  async function respond(approve: boolean) {
    try {
      await respondFor(request, requestId, approve)
    } finally {
      useApprovalQueueStore.getState().clearRequest(requestId)
    }
  }

  const title =
    request.kind === 'connect'
      ? t('mcpApproval.connectTitle')
      : request.kind === 'control'
        ? t('mcpApproval.controlTitle')
        : request.kind === 'scope'
          ? t('mcpApproval.scopeTitle')
          : t('mcpApproval.commandTitle')

  return (
    <Dialog open onClose={() => respond(false)} title={title}>
      <div className="flex flex-col gap-4 w-100">
        {request.kind === 'connect' && (
          <p className="m-0 text-[13px] text-fg2 leading-relaxed">
            {t('mcpApproval.connectBody', { hostName: request.payload.hostName })}
          </p>
        )}
        {request.kind === 'control' && (
          <p className="m-0 text-[13px] text-fg2 leading-relaxed">
            {t('mcpApproval.controlBody', { sessionId: request.payload.sessionId })}
          </p>
        )}
        {request.kind === 'scope' && (
          <p className="m-0 text-[13px] text-fg2 leading-relaxed">
            {t('mcpApproval.scopeBody', {
              count: request.payload.hostNames.length,
              hostNames: request.payload.hostNames.join(', '),
            })}
          </p>
        )}
        {request.kind === 'command' && (
          <div className="flex flex-col gap-2.5">
            <div className="flex items-center gap-2">
              <Chip tone={request.payload.risk === 'high' ? 'red' : request.payload.risk === 'medium' ? 'neutral' : 'green'}>
                {request.payload.risk}
              </Chip>
              {request.payload.uncertain && <Chip tone="red">{t('mcpApproval.commandUncertain')}</Chip>}
            </div>
            <code className="block px-2.5 py-2 rounded-md bg-inputbg text-[12px] font-mono whitespace-pre-wrap break-all">
              {request.payload.command}
            </code>
            {request.payload.guardedCmd !== request.payload.command && (
              <div className="text-[11px] text-fg3">
                <div>{t('mcpApproval.commandGuarded')}</div>
                <code className="block px-2.5 py-2 rounded-md bg-inputbg text-[12px] font-mono whitespace-pre-wrap break-all">
                  {request.payload.guardedCmd}
                </code>
              </div>
            )}
            {request.payload.reasons.length > 0 && (
              <ul className="m-0 pl-4.5 text-[12px] text-fg2 leading-relaxed">
                {request.payload.reasons.map((reason) => (
                  <li key={reason}>{reason}</li>
                ))}
              </ul>
            )}
          </div>
        )}
        <div className="flex justify-end gap-2.5">
          <Button onClick={() => respond(false)}>{t('mcpApproval.reject')}</Button>
          <Button variant="primary" onClick={() => respond(true)}>
            {t('mcpApproval.approve')}
          </Button>
        </div>
      </div>
    </Dialog>
  )
}

function respondFor(request: ApprovalRequest, requestId: string, approve: boolean): Promise<void> {
  switch (request.kind) {
    case 'connect':
      return respondConnectApproval(requestId, approve)
    case 'control':
      return respondControlApproval(requestId, approve)
    case 'scope':
      return respondConnectionScopeApproval(requestId, approve)
    case 'command':
      return respondCommandApproval(requestId, approve)
  }
}
