import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { Host } from '../../../entities/host'
import { HostFormDialog } from '../../../features/host-crud'
import { Button, Spinner } from '../../../shared/ui'
import { useSftpStore } from '../model/store'
import { HostPickerGrid } from './HostPickerGrid'

/** Shown in place of the remote pane whenever the SFTP page has no session.
 * idle: connect to a saved host (via the picker) or register+connect a new
 * one. picker: a home-page-style host card grid (G2). `view` is local UI
 * navigation state, not store state -- a successful connect unmounts this
 * component entirely, so the next disconnect naturally restarts at idle. */
export function ConnectGate() {
  const { t } = useTranslation()
  const connecting = useSftpStore((s) => s.connecting)
  const gateError = useSftpStore((s) => s.gateError)
  const connect = useSftpStore((s) => s.connect)
  const [view, setView] = useState<'idle' | 'picker'>('idle')
  const [dialogOpen, setDialogOpen] = useState(false)

  function handleSaved(host: Host) {
    setDialogOpen(false)
    void connect(host)
  }

  if (connecting) {
    return (
      <div className="flex-1 basis-0 min-w-0 rounded-xl bg-surface border border-line flex flex-col items-center justify-center gap-2 text-fg2 text-[13px]">
        <Spinner size={20} />
        {t('common.connecting')}
      </div>
    )
  }

  return (
    <div className="flex-1 basis-0 min-w-0 min-h-0 rounded-xl bg-surface border border-line overflow-hidden flex flex-col">
      {view === 'picker' ? (
        <HostPickerGrid onBack={() => setView('idle')} onRegister={() => setDialogOpen(true)} />
      ) : (
        <div className="flex-1 flex flex-col items-center justify-center gap-4.5 p-8">
          <div className="w-14 h-14 rounded-2xl bg-surface2 flex items-center justify-center text-[22px] font-mono text-fg3">⇅</div>
          <div className="flex flex-col items-center gap-1.5">
            <div className="text-[15.5px] font-bold">{t('sftp.gate.notConnected')}</div>
            <div className="text-[12.5px] text-fg2 text-center leading-relaxed">{t('sftp.gate.desc')}</div>
          </div>
          <div className="flex gap-2.5">
            <Button variant="primary" onClick={() => setView('picker')}>
              {t('common.connect')}
            </Button>
            <Button onClick={() => setDialogOpen(true)}>{t('sftp.gate.registerHost')}</Button>
          </div>
          {gateError && <div className="text-[11.5px] text-red">{t('sftp.gate.previousConnectFailed', { error: gateError })}</div>}
        </div>
      )}
      {dialogOpen && <HostFormDialog open onClose={() => setDialogOpen(false)} onSaved={handleSaved} />}
    </div>
  )
}
