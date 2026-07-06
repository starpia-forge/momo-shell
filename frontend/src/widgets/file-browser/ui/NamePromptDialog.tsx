import { useEffect, useState } from 'react'
import { Button, Dialog, TextInput } from '../../../shared/ui'

interface NamePromptDialogProps {
  open: boolean
  title: string
  initialValue?: string
  onConfirm: (name: string) => void
  onClose: () => void
}

/** Single-field name prompt shared by "새 폴더" and "이름 변경". */
export function NamePromptDialog({ open, title, initialValue = '', onConfirm, onClose }: NamePromptDialogProps) {
  const [value, setValue] = useState(initialValue)

  useEffect(() => {
    if (open) setValue(initialValue)
  }, [open, initialValue])

  function submit() {
    const trimmed = value.trim()
    if (!trimmed) return
    onConfirm(trimmed)
    onClose()
  }

  return (
    <Dialog open={open} onClose={onClose} title={title}>
      <form
        className="flex flex-col gap-3 min-w-60"
        onSubmit={(e) => {
          e.preventDefault()
          submit()
        }}
      >
        <TextInput autoFocus value={value} onChange={(e) => setValue(e.target.value)} />
        <div className="flex justify-end gap-2">
          <Button type="button" onClick={onClose}>
            취소
          </Button>
          <Button type="submit" variant="primary" disabled={!value.trim()}>
            확인
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
