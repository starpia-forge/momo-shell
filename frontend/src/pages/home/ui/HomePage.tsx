import type { Host } from '../../../entities/host'
import { SavedHostsGrid } from './SavedHostsGrid'
import { SharedHostsGrid } from './SharedHostsGrid'
import './HomePage.css'

interface HomePageProps {
  onConnect: (host: Host, sessionId: string) => void
  onConnectShared: (name: string, address: string, sessionId: string) => void
}

export function HomePage({ onConnect, onConnectShared }: HomePageProps) {
  return (
    <div className="home-page">
      <SavedHostsGrid onConnect={onConnect} />
      <SharedHostsGrid onConnect={onConnect} onConnectShared={onConnectShared} />
    </div>
  )
}
