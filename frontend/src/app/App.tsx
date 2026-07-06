import { useEffect } from 'react'
import { registerFileDropRouter } from '../features/file-upload'
import { WorkspacePage } from '../pages/workspace'
import { ToastHost } from '../shared/ui'
import { useSettingsStore } from '../entities/settings'
import { registerGlobalShortcuts } from './keyboard'
import { initSettingsBridge } from './settingsBridge'
import './styles/main.css'

export default function App() {
  useEffect(() => registerGlobalShortcuts(), [])
  useEffect(() => registerFileDropRouter(), [])
  useEffect(() => initSettingsBridge(), [])
  useEffect(() => {
    void useSettingsStore.getState().load()
  }, [])
  return (
    <>
      <WorkspacePage />
      <ToastHost />
    </>
  )
}
