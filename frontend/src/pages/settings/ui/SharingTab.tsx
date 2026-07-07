import { useEffect, useState, type KeyboardEvent } from 'react'
import { getDeviceName, setDeviceName, describeShareError } from '../../../shared/api/share'
import { TextInput, useToastStore } from '../../../shared/ui'

export function SharingTab() {
  const [name, setName] = useState('')

  useEffect(() => {
    void getDeviceName().then(setName)
  }, [])

  function commit() {
    setDeviceName(name.trim())
      .then(() => useToastStore.getState().push('장치 이름을 저장했습니다'))
      .catch((err) => useToastStore.getState().push(describeShareError(err)))
  }

  function handleKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') e.currentTarget.blur()
  }

  return (
    <section>
      <h2 className="text-[14px] font-semibold mb-3">공유</h2>
      <div className="flex items-center justify-between py-2.5 border-b border-line">
        <div>
          <div className="text-[13px]">장치 이름</div>
          <div className="text-[11px] text-fg2">다음 mDNS 광고·페어링부터 적용됩니다</div>
        </div>
        <TextInput className="w-50" value={name} onChange={(e) => setName(e.target.value)} onBlur={commit} onKeyDown={handleKeyDown} />
      </div>
    </section>
  )
}
