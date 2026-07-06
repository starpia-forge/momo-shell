import { useEffect } from 'react'
import { registerFileDropRouter } from '../features/file-upload'
import { WorkspacePage } from '../pages/workspace'
import { ToastHost } from '../shared/ui'
import { registerGlobalShortcuts } from './keyboard'
import './styles/main.css'

export default function App() {
  useEffect(() => registerGlobalShortcuts(), [])
  useEffect(() => registerFileDropRouter(), [])
  return (
    <>
      <WorkspacePage />
      <ToastHost />
    </>
  )
}
