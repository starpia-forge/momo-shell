import { useEffect } from 'react'
import { WorkspacePage } from '../pages/workspace'
import { registerGlobalShortcuts } from './keyboard'
import './styles/global.css'

export default function App() {
  useEffect(() => registerGlobalShortcuts(), [])
  return <WorkspacePage />
}
