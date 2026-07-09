import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { listMCPClients, revokeMCPClient, type MCPClient } from '../../../shared/api/mcp'
import { Button, useToastStore } from '../../../shared/ui'

export function MCPClientsTab() {
  const { t } = useTranslation()
  const [clients, setClients] = useState<MCPClient[]>([])

  function reload() {
    void listMCPClients().then(setClients)
  }

  useEffect(() => {
    reload()
  }, [])

  function revoke(client: MCPClient) {
    revokeMCPClient(client.id)
      .then(() => {
        useToastStore.getState().push(t('settings.mcp.revokeSuccess', { name: client.name }), 'success')
        reload()
      })
      .catch(() => useToastStore.getState().push(t('settings.mcp.revokeFailed'), 'error'))
  }

  return (
    <section className="flex flex-col gap-2">
      <h2 className="text-[20px] font-bold mb-1">{t('settings.mcp.title')}</h2>
      <p className="m-0 text-[12px] text-fg3">{t('settings.mcp.desc')}</p>
      {clients.length === 0 ? (
        <div className="text-[12px] text-fg2 py-3.5">{t('settings.mcp.noClients')}</div>
      ) : (
        <ul className="flex flex-col gap-1 m-0 p-0 list-none">
          {clients.map((c) => (
            <li
              key={c.id}
              className="flex items-center justify-between gap-2 py-3.5 border-b border-line"
            >
              <div className="flex flex-col gap-1">
                <span className="text-[14px] font-medium">
                  {c.name}
                  {c.revoked && (
                    <span className="ml-2 text-[11px] font-normal text-fg3">
                      {t('settings.mcp.revoked')}
                    </span>
                  )}
                </span>
                <span className="text-[12px] text-fg3">
                  {t('settings.mcp.pairedAt', { time: new Date(c.pairedAt * 1000).toLocaleString() })}
                  {c.lastSeenAt != null &&
                    ` · ${t('settings.mcp.lastSeen', { time: new Date(c.lastSeenAt * 1000).toLocaleString() })}`}
                </span>
              </div>
              {!c.revoked && (
                <Button variant="danger" onClick={() => revoke(c)}>
                  {t('settings.mcp.revoke')}
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
