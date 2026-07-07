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
  function choose(policy: ConflictPolicy) {
    onChoice(policy)
    onClose()
  }

  return (
    <Dialog open={open} onClose={onClose} title="같은 이름의 파일이 있습니다">
      <div className="conflict-dialog flex flex-col gap-4 w-110">
        <p className="m-0 text-[13px] text-fg2 leading-relaxed">
          다음 {conflicts.length}개 파일이 이미 존재합니다. 어떻게 처리할까요?
        </p>
        <ul className="m-0 max-h-35 list-none p-0 flex flex-col gap-1 overflow-y-auto font-mono text-[12px] text-fg2">
          {conflicts.slice(0, VISIBLE_LIMIT).map((name) => (
            <li key={name} className="px-3.5 py-2 rounded-md bg-inputbg">
              {name}
            </li>
          ))}
          {conflicts.length > VISIBLE_LIMIT && <li className="px-3.5 py-2">외 {conflicts.length - VISIBLE_LIMIT}개...</li>}
        </ul>
        <div className="flex justify-end gap-2.5">
          <Button type="button" onClick={onClose}>
            취소
          </Button>
          <Button type="button" onClick={() => choose('skip')}>
            건너뛰기
          </Button>
          <Button type="button" variant="primary" onClick={() => choose('overwrite')}>
            덮어쓰기
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
