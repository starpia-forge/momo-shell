import { subscribe, topics, type MCPCommandStatePayload, type MCPDelegationPayload } from '../../../shared/api/events'
import { listConnectionScopes, listDelegations } from '../../../shared/api/mcp'
import { useDelegationStore } from '../model/delegations'

/** Wires mcp:delegation + mcp:cmd-state into the delegation store, seeded
 * with whatever's already active/granted on load (both events only report
 * transitions from the moment of subscription forward --
 * registerTransferCenter's pattern). Call once at app startup. */
export function registerDelegations(): () => void {
  void listDelegations().then((delegations) => useDelegationStore.getState().replaceAll(delegations))
  void listConnectionScopes().then((scopes) => useDelegationStore.getState().setScopes(scopes))

  const unsubDelegation = subscribe<MCPDelegationPayload>(topics.mcpDelegation(), (payload) => {
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
    // A grant/release/kill/expire changes a scope's live active count (B5b's
    // "N/cap" display) -- re-fetch rather than adding a separate scope
    // topic, since the server already computes this fresh on every read.
    void listConnectionScopes().then((scopes) => useDelegationStore.getState().setScopes(scopes))
  })

  const unsubCmdState = subscribe<MCPCommandStatePayload>(topics.mcpCommandState(), (payload) => {
    useDelegationStore.getState().setCmdState(payload)
  })

  return () => {
    unsubDelegation()
    unsubCmdState()
  }
}
