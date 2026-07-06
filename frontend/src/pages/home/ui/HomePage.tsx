import type { Host } from '../../../entities/host'
import { SavedHostsGrid } from './SavedHostsGrid'
import { SharedHostsGrid } from './SharedHostsGrid'

interface HomePageProps {
  onConnect: (host: Host, sessionId: string) => void
  onConnectShared: (name: string, address: string, sessionId: string) => void
}

export function HomePage({ onConnect, onConnectShared }: HomePageProps) {
  return (
    <div className="home-page h-full overflow-y-auto p-4 px-5 bg-canvas text-fg flex flex-col gap-6">
      <SavedHostsGrid onConnect={onConnect} />
      <SharedHostsGrid onConnect={onConnect} onConnectShared={onConnectShared} />
    </div>
  )
}
