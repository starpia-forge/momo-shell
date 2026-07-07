import { useTranslation } from 'react-i18next'
import type { ConflictPolicy } from '../../api/transfer'
import { Button } from '../Button/Button'
import { Dialog } from '../Dialog/Dialog'

interface ConflictDialogProps {
  open: boolean
  /** Filenames that already exist at the upload destination. */
  conflicts: string[]
  onChoice: (policy: ConflictPolicy) => void
  onClose: () => void
}

const VISIBLE_LIMIT = 10

/** Prompts for an overwrite/rename/skip policy before an upload that would
 * otherwise silently overwrite existing remote files. The chosen policy
 * applies to the whole batch (files without a conflict are unaffected by
 * whichever policy is picked). */
export function ConflictDialog({ open, conflicts, onChoice, onClose }: ConflictDialogProps) {
  const { t } = useTranslation()

  function choose(policy: ConflictPolicy) {
    onChoice(policy)
    onClose()
  }

  return (
    <Dialog open={open} onClose={onClose} title={t('conflictDialog.title')}>
      <div className="conflict-dialog flex flex-col gap-4 w-110">
        <p className="m-0 text-[13px] text-fg2 leading-relaxed">{t('conflictDialog.body', { count: conflicts.length })}</p>
        <ul className="m-0 max-h-35 list-none p-0 flex flex-col gap-1 overflow-y-auto font-mono text-[12px] text-fg2">
          {conflicts.slice(0, VISIBLE_LIMIT).map((name) => (
            <li key={name} className="px-3.5 py-2 rounded-md bg-inputbg">
              {name}
            </li>
          ))}
          {conflicts.length > VISIBLE_LIMIT && (
            <li className="px-3.5 py-2">{t('conflictDialog.moreCount', { count: conflicts.length - VISIBLE_LIMIT })}</li>
          )}
        </ul>
        <div className="flex justify-end gap-2.5">
          <Button type="button" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button type="button" onClick={() => choose('skip')}>
            {t('conflictDialog.skip')}
          </Button>
          <Button type="button" variant="primary" onClick={() => choose('overwrite')}>
            {t('conflictDialog.overwrite')}
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
