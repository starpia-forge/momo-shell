import { useSettingsStore, type Theme } from '../../../entities/settings'
import { useToastStore } from '../../../shared/ui'

const OPTIONS: { value: Theme; label: string }[] = [
  { value: 'dark', label: '다크' },
  { value: 'light', label: '라이트' },
]

export function AppearanceTab() {
  const theme = useSettingsStore((s) => s.theme)

  function handleChange(value: Theme) {
    useSettingsStore
      .getState()
      .setTheme(value)
      .catch(() => useToastStore.getState().push('테마 변경에 실패했습니다'))
  }

  return (
    <section>
      <h2 className="text-[14px] font-semibold mb-3">모양</h2>
      <div className="flex items-center justify-between py-2.5 border-b border-line">
        <span className="text-[13px]">테마</span>
        <div className="flex gap-3">
          {OPTIONS.map((opt) => (
            <label key={opt.value} className="flex items-center gap-1.5 text-[13px] cursor-pointer">
              <input type="radio" name="theme" checked={theme === opt.value} onChange={() => handleChange(opt.value)} />
              {opt.label}
            </label>
          ))}
        </div>
      </div>
    </section>
  )
}
