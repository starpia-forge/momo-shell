import { useTranslation } from 'react-i18next'
import { cn } from '../../../shared/lib/cn'
import { isActiveTask, sortedTasks, useTransferCenterStore } from '../model/store'

/** Tab-bar pill that toggles the TransferCenter popover; mirrors its task
 * count. Both read the same store so they stay in sync without coupling. */
export function TransferBadge() {
  const { t } = useTranslation()
  const tasks = useTransferCenterStore((s) => s.tasks)
  const open = useTransferCenterStore((s) => s.open)

  const list = sortedTasks(tasks)
  const activeCount = list.filter(isActiveTask).length

  if (list.length === 0) return null

  return (
    <button
      className={cn(
        'flex-none flex items-center gap-2 h-8.5 px-3.5 rounded-full border text-[12.5px] font-medium cursor-pointer',
        activeCount > 0
          ? 'bg-accent/16 border-accent text-accent-text font-bold'
          : 'bg-surface2 border-line text-fg',
        open && 'brightness-110',
      )}
      onClick={() => useTransferCenterStore.getState().toggleOpen()}
    >
      <span className="font-mono font-bold">↑↓</span>
      <span>{t('transfer.badgeLabel', { count: activeCount > 0 ? activeCount : list.length })}</span>
    </button>
  )
}
