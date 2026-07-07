import { useState, type KeyboardEvent } from 'react'
import { useSettingsStore } from '../../../entities/settings'
import { IconButton, TextInput, useToastStore } from '../../../shared/ui'

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
    <section className="flex flex-col gap-2">
      <h2 className="text-[20px] font-bold mb-1">터미널</h2>
      <div className="flex items-center justify-between py-3.5 border-b border-line">
        <span className="text-[14px] font-medium">폰트 크기</span>
        <div className="flex items-center gap-0.5 p-0.75 rounded-md bg-inputbg border border-line">
          <IconButton size={30} className="text-[15px]" onClick={() => { const s = useSettingsStore.getState(); s.setFontSize(s.fontSize - 1) }}>
            −
          </IconButton>
          <span className="w-11 text-center font-mono text-[13px]">{fontSize}</span>
          <IconButton size={30} className="text-[15px]" onClick={() => { const s = useSettingsStore.getState(); s.setFontSize(s.fontSize + 1) }}>
            ＋
          </IconButton>
        </div>
      </div>
      <div className="flex items-center justify-between py-3.5 border-b border-line">
        <div className="flex flex-col gap-1">
          <span className="text-[14px] font-medium">스크롤백</span>
          <span className="text-[12px] text-fg3">1,000 ~ 100,000줄</span>
        </div>
        <TextInput
          className="w-25 text-right font-mono"
          value={scrollbackDraft}
          onChange={(e) => setScrollbackDraft(e.target.value)}
          onBlur={commitScrollback}
          onKeyDown={handleScrollbackKeyDown}
        />
      </div>
    </section>
  )
}
