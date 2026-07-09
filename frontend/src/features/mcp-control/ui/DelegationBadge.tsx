import { useTranslation } from 'react-i18next'
import { cn } from '../../../shared/lib/cn'
import { useDelegationStore } from '../model/delegations'

/** Tab-bar pill that toggles the DelegationPanel popover; mirrors its
 * active-delegation count. TransferBadge's structural twin. */
export function DelegationBadge() {
  const { t } = useTranslation()
  const delegations = useDelegationStore((s) => s.delegations)
  const open = useDelegationStore((s) => s.open)

  const count = Object.keys(delegations).length
  if (count === 0) return null

  return (
    <button
      className={cn(
        'flex-none flex items-center gap-2 h-8.5 px-3.5 rounded-full border text-[12.5px] font-medium cursor-pointer',
        'bg-accent/16 border-accent text-accent-text font-bold',
        open && 'brightness-110',
      )}
      onClick={() => useDelegationStore.getState().toggleOpen()}
    >
      <span className="font-mono font-bold">●</span>
      <span>{t('delegation.badgeLabel', { count })}</span>
    </button>
  )
}
