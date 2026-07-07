import { useState, type KeyboardEvent } from 'react'
import { useSettingsStore } from '../../../entities/settings'
import { Button, TextInput, useToastStore } from '../../../shared/ui'

export function TerminalTab() {
  const fontSize = useSettingsStore((s) => s.fontSize)
  const scrollback = useSettingsStore((s) => s.scrollback)
  const [scrollbackDraft, setScrollbackDraft] = useState(String(scrollback))

  function commitScrollback() {
    const value = Number(scrollbackDraft)
    if (!Number.isFinite(value)) {
      setScrollbackDraft(String(scrollback))
      return
    }
    useSettingsStore
      .getState()
      .setScrollback(value)
      .then(() => setScrollbackDraft(String(useSettingsStore.getState().scrollback)))
      .catch(() => {
        useToastStore.getState().push('스크롤백 설정 변경에 실패했습니다', 'error')
        setScrollbackDraft(String(scrollback))
      })
  }

  function handleScrollbackKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') e.currentTarget.blur()
  }

  return (
    <section>
      <h2 className="text-[14px] font-semibold mb-3">터미널</h2>
      <div className="flex items-center justify-between py-2.5 border-b border-line">
        <span className="text-[13px]">폰트 크기</span>
        <div className="flex items-center gap-2">
          <Button onClick={() => { const s = useSettingsStore.getState(); s.setFontSize(s.fontSize - 1) }}>−</Button>
          <span className="text-[13px] w-8 text-center">{fontSize}</span>
          <Button onClick={() => { const s = useSettingsStore.getState(); s.setFontSize(s.fontSize + 1) }}>+</Button>
        </div>
      </div>
      <div className="flex items-center justify-between py-2.5 border-b border-line">
        <div>
          <div className="text-[13px]">스크롤백</div>
          <div className="text-[11px] text-fg2">1,000 ~ 100,000줄</div>
        </div>
        <TextInput
          className="w-25 text-right"
          value={scrollbackDraft}
          onChange={(e) => setScrollbackDraft(e.target.value)}
          onBlur={commitScrollback}
          onKeyDown={handleScrollbackKeyDown}
        />
      </div>
    </section>
  )
}
