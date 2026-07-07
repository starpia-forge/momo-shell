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
    <Dialog open={open} onClose={onClose} title="이름 충돌">
      <div className="conflict-dialog flex flex-col gap-3 min-w-70 max-w-90">
        <p className="m-0 text-[13px]">다음 {conflicts.length}개 파일이 이미 존재합니다:</p>
        <ul className="m-0 max-h-35 list-disc overflow-y-auto pl-[18px] text-[12px] text-fg2">
          {conflicts.slice(0, VISIBLE_LIMIT).map((name) => (
            <li key={name}>{name}</li>
          ))}
          {conflicts.length > VISIBLE_LIMIT && <li>외 {conflicts.length - VISIBLE_LIMIT}개...</li>}
        </ul>
        <div className="flex justify-end gap-2">
          <Button type="button" onClick={onClose}>
            취소
          </Button>
          <Button type="button" onClick={() => choose('skip')}>
            건너뛰기
          </Button>
          <Button type="button" onClick={() => choose('rename')}>
            이름 변경
          </Button>
          <Button type="button" variant="primary" onClick={() => choose('overwrite')}>
            덮어쓰기
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
