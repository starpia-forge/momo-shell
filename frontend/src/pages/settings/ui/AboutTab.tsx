import { useEffect, useState } from 'react'
import { getAppInfo } from '../../../shared/api/settings'

export function AboutTab() {
  const [version, setVersion] = useState('')

  useEffect(() => {
    void getAppInfo().then((info) => setVersion(info.version))
  }, [])

  return (
    <section>
      <h2 className="text-[14px] font-semibold mb-3">정보</h2>
      <div className="flex items-center justify-between py-2.5 border-b border-line">
        <span className="text-[13px]">momo-shell</span>
        <span className="text-[13px] text-fg2">{version}</span>
      </div>
    </section>
  )
}
