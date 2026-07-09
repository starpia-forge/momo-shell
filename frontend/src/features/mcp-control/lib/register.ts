import { subscribe, topics, type MCPDelegationPayload } from '../../../shared/api/events'
import { listDelegations } from '../../../shared/api/mcp'
import { useDelegationStore } from '../model/delegations'

/** Wires mcp:delegation into the delegation store, seeded with whatever's
 * already active on load (mcp:delegation only reports transitions from the
 * moment of subscription forward -- registerTransferCenter's pattern). Call
 * once at app startup. */
export function registerDelegations(): () => void {
  void listDelegations().then((delegations) => useDelegationStore.getState().replaceAll(delegations))

  const unsub = subscribe<MCPDelegationPayload>(topics.mcpDelegation(), (payload) => {
    if (payload.state === 'delegated') {
      useDelegationStore.getState().setDelegation({
        sessionId: payload.sessionId,
        clientId: payload.clientId,
        aiCreated: payload.aiCreated,
        grantedAt: Date.now() / 1000,
      })
    } else {
      useDelegationStore.getState().removeDelegation(payload.sessionId)
    }
  })

  return unsub
}
