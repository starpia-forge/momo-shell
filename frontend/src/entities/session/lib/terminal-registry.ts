import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebglAddon } from '@xterm/addon-webgl'
import { CanvasAddon } from '@xterm/addon-canvas'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { SearchAddon } from '@xterm/addon-search'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import '@xterm/xterm/css/xterm.css'

import {
  createLocalSession,
  writeSession,
  resizeSession,
  closeSession,
  type CreateLocalSessionOpts,
} from '../../../shared/api/session'
import { subscribe, topics, type SessionStatePayload, type SessionClosedPayload } from '../../../shared/api/events'
import { b64ToBytes } from '../../../shared/lib/base64'
import { useSessionStore } from '../model/store'

// The xterm.js instance is owned here, outside React, in a detached host
// div. TerminalPane only ever attaches/detaches that div -- it never
// creates or destroys the Terminal -- so moving a pane (Phase 3 split
// drag/dock) is a DOM re-parent, not a session restart. Event subscriptions
// likewise live for the session's lifetime, not the component's mount time,
// so output isn't lost while a pane is unmounted.
interface TerminalEntry {
  term: Terminal
  fit: FitAddon
  host: HTMLDivElement
  unsubs: Array<() => void>
  attached: HTMLElement | null
}

const registry = new Map<string, TerminalEntry>()

const DEFAULT_FONT_SIZE = 14
const MIN_FONT_SIZE = 8
const MAX_FONT_SIZE = 32
let currentFontSize = DEFAULT_FONT_SIZE

// Cell metrics depend on the font actually being loaded; re-fit every
// attached terminal once it is, correcting any fallback-font sizing done
// before that point.
if (typeof document !== 'undefined' && document.fonts) {
  document.fonts.ready.then(() => {
    registry.forEach((entry) => fitNow(entry))
  })
}

export async function openLocalSession(opts: CreateLocalSessionOpts): Promise<string> {
  const info = await createLocalSession(opts)
  const { id } = info

  const term = new Terminal({
    scrollback: 10000,
    allowProposedApi: true,
    fontFamily: '"JetBrains Mono", "Cascadia Code", Consolas, monospace',
    fontSize: currentFontSize,
    theme: { background: '#1b2636' },
  })

  const fit = new FitAddon()
  term.loadAddon(fit)
  term.loadAddon(new SearchAddon())
  term.loadAddon(new WebLinksAddon())
  term.loadAddon(new Unicode11Addon())
  term.unicode.activeVersion = '11'

  try {
    const webgl = new WebglAddon()
    term.loadAddon(webgl)
    webgl.onContextLoss(() => {
      webgl.dispose()
      term.loadAddon(new CanvasAddon())
    })
  } catch {
    term.loadAddon(new CanvasAddon())
  }

  const host = document.createElement('div')
  host.style.width = '100%'
  host.style.height = '100%'
  term.open(host)

  const unsubs: Array<() => void> = []

  const dataDisposable = term.onData((data) => {
    void writeSession(id, new TextEncoder().encode(data))
  })
  unsubs.push(() => dataDisposable.dispose())

  const resizeDisposable = term.onResize(({ cols, rows }) => {
    void resizeSession(id, cols, rows)
  })
  unsubs.push(() => resizeDisposable.dispose())

  unsubs.push(
    subscribe<string>(topics.sessionData(id), (b64) => {
      term.write(b64ToBytes(b64))
    })
  )
  unsubs.push(
    subscribe<SessionStatePayload>(topics.sessionState(id), (payload) => {
      useSessionStore.getState().setState(id, payload.state, { error: payload.error })
    })
  )
  unsubs.push(
    subscribe<SessionClosedPayload>(topics.sessionClosed(id), (payload) => {
      useSessionStore.getState().setState(id, 'closed', { exitCode: payload.exitCode })
    })
  )

  registry.set(id, { term, fit, host, unsubs, attached: null })

  useSessionStore.getState().upsert({
    id: info.id,
    shell: info.shell,
    cols: info.cols,
    rows: info.rows,
    state: 'running',
  })

  return id
}

export function attach(id: string, container: HTMLElement): void {
  const entry = registry.get(id)
  if (!entry || entry.attached === container) return
  container.appendChild(entry.host)
  entry.attached = container
  fitNow(entry)
}

export function detach(id: string): void {
  const entry = registry.get(id)
  if (!entry) return
  entry.host.remove()
  entry.attached = null
}

export function fitSession(id: string): void {
  const entry = registry.get(id)
  if (entry) fitNow(entry)
}

function fitNow(entry: TerminalEntry): void {
  if (!entry.attached) return
  const { clientWidth, clientHeight } = entry.attached
  if (clientWidth === 0 || clientHeight === 0) return
  entry.fit.fit()
}

export function getFontSize(): number {
  return currentFontSize
}

export function setFontSize(size: number): void {
  currentFontSize = Math.min(MAX_FONT_SIZE, Math.max(MIN_FONT_SIZE, size))
  registry.forEach((entry) => {
    entry.term.options.fontSize = currentFontSize
    fitNow(entry)
  })
}

export function resetFontSize(): void {
  setFontSize(DEFAULT_FONT_SIZE)
}

export function disposeSession(id: string): void {
  const entry = registry.get(id)
  if (!entry) return
  entry.unsubs.forEach((unsub) => unsub())
  entry.term.dispose()
  void closeSession(id)
  registry.delete(id)
  useSessionStore.getState().remove(id)
}
