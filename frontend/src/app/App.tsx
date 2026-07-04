import { useEffect } from 'react'
import { registerFileDropRouter } from '../features/file-upload'
import { WorkspacePage } from '../pages/workspace'
import { registerGlobalShortcuts } from './keyboard'
import './styles/global.css'

export default function App() {
  useEffect(() => registerGlobalShortcuts(), [])
  useEffect(() => registerFileDropRouter(), [])
  return <WorkspacePage />
}
