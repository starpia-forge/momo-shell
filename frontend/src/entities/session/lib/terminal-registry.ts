import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebglAddon } from '@xterm/addon-webgl'
import { CanvasAddon } from '@xterm/addon-canvas'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { SearchAddon, type ISearchOptions } from '@xterm/addon-search'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import '@xterm/xterm/css/xterm.css'

import {
  createLocalSession,
  createSSHSession,
  createSSHDirectSession,
  writeSession,
  resizeSession,
  closeSession,
  type CreateLocalSessionOpts,
  type CreateSSHDirectSessionOpts,
} from '../../../shared/api/session'
import {
  subscribe,
  topics,
  type SessionStatePayload,
  type SessionClosedPayload,
  type SessionHostKeyPayload,
} from '../../../shared/api/events'
import { b64ToBytes } from '../../../shared/lib/base64'
import { useSessionStore } from '../model/store'
import { useHostKeyPromptStore } from '../model/hostKeyPrompts'
import { parseOsc7Path } from './osc7'
import { hexToRgba, readAccentColor, readTerminalFontFamily, readTerminalTheme } from './terminal-theme'

// The xterm.js instance is owned here, outside React, in a detached host
// div. TerminalPane only ever attaches/detaches that div -- it never
// creates or destroys the Terminal -- so moving a pane (Phase 3 split
// drag/dock) is a DOM re-parent, not a session restart. Event subscriptions
// likewise live for the session's lifetime, not the component's mount time,
// so output isn't lost while a pane is unmounted.
interface TerminalEntry {
  term: Terminal
  fit: FitAddon
  search: SearchAddon
  host: HTMLDivElement
  unsubs: Array<() => void>
  attached: HTMLElement | null
  closed: boolean
}

const registry = new Map<string, TerminalEntry>()

const DEFAULT_FONT_SIZE = 14
const MIN_FONT_SIZE = 8
const MAX_FONT_SIZE = 32
let currentFontSize = DEFAULT_FONT_SIZE

const DEFAULT_SCROLLBACK = 10000
const MIN_SCROLLBACK = 1000
const MAX_SCROLLBACK = 100000
let currentScrollback = DEFAULT_SCROLLBACK

// Cell metrics depend on the font actually being loaded; re-fit every
// attached terminal once it is, correcting any fallback-font sizing done
// before that point.
if (typeof document !== 'undefined' && document.fonts) {
  document.fonts.ready.then(() => {
    registry.forEach((entry) => fitNow(entry))
  })
}

// createTerminalEntry builds the xterm.js instance + addons + detached host
// div + the event wiring every session needs (input, resize, output,
// state/closed, and the SSH host-key prompt -- harmless to wire for local
// sessions too, since the backend simply never publishes that topic for
// them). openLocalSession/openSSHSession differ only in which RPC they call
// and what they seed the session store with.
function createTerminalEntry(id: string): void {
  const term = new Terminal({
    scrollback: currentScrollback,
    allowProposedApi: true,
    fontFamily: readTerminalFontFamily(),
    fontSize: currentFontSize,
    theme: readTerminalTheme(),
  })

  const fit = new FitAddon()
  term.loadAddon(fit)
  const search = new SearchAddon()
  term.loadAddon(search)
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
    if (registry.get(id)?.closed) return
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
      if (payload.state === 'closed' || payload.state === 'error') {
        const entry = registry.get(id)
        if (entry) entry.closed = true
      }
    })
  )
  unsubs.push(
    subscribe<SessionClosedPayload>(topics.sessionClosed(id), (payload) => {
      useSessionStore.getState().setState(id, 'closed', { exitCode: payload.exitCode })
      const entry = registry.get(id)
      if (entry) entry.closed = true
    })
  )
  unsubs.push(
    subscribe<SessionHostKeyPayload>(topics.sessionHostKey(id), (payload) => {
      useHostKeyPromptStore.getState().setPrompt(id, payload)
    })
  )

  // OSC 7 ("file://host/path") reports the shell's cwd -- used to default
  // pane file-drop uploads to "wherever the prompt currently is" instead of
  // always the SFTP home directory.
  const oscDisposable = term.parser.registerOscHandler(7, (data) => {
    const path = parseOsc7Path(data)
    if (path) useSessionStore.getState().setCwd(id, path)
    return true
  })
  unsubs.push(() => oscDisposable.dispose())

  registry.set(id, { term, fit, search, host, unsubs, attached: null, closed: false })
}

export async function openLocalSession(opts: CreateLocalSessionOpts): Promise<string> {
  const info = await createLocalSession(opts)
  createTerminalEntry(info.id)

  useSessionStore.getState().upsert({
    id: info.id,
    kind: 'local',
    shell: info.shell ?? '',
    cols: info.cols,
    rows: info.rows,
    state: 'running',
  })

  return info.id
}

export async function openSSHSession(hostId: string, cols: number, rows: number): Promise<string> {
  const info = await createSSHSession({ hostId, cols, rows })
  createTerminalEntry(info.id)

  useSessionStore.getState().upsert({
    id: info.id,
    kind: 'ssh',
    hostId: info.hostId,
    shell: '',
    cols: info.cols,
    rows: info.rows,
    state: 'connecting',
  })

  return info.id
}

// openSSHDirectSession is openSSHSession's counterpart for a host with no
// saved Host row (e.g. a peer's shared host) -- info.hostId comes back ""
// since there is no saved host to reference.
export async function openSSHDirectSession(opts: CreateSSHDirectSessionOpts): Promise<string> {
  const info = await createSSHDirectSession(opts)
  createTerminalEntry(info.id)

  useSessionStore.getState().upsert({
    id: info.id,
    kind: 'ssh',
    hostId: info.hostId,
    shell: '',
    cols: info.cols,
    rows: info.rows,
    state: 'connecting',
  })

  return info.id
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

/** Search-match highlight colors come from --theme-accent, not a fixed hex --
 * xterm's decorations option only accepts solid colors, so this is computed
 * per-call rather than read once (accent can change at runtime). */
function searchDecorations(): ISearchOptions {
  const accent = readAccentColor()
  const match = hexToRgba(accent, 0.3)
  const active = hexToRgba(accent, 0.55)
  return {
    decorations: {
      matchBackground: match,
      matchOverviewRuler: match,
      activeMatchBackground: active,
      activeMatchColorOverviewRuler: active,
    },
  }
}

/** Returns whether a match was found. Highlights every match, not just the current one. */
export function searchSession(id: string, term: string, direction: 'next' | 'prev'): boolean {
  const entry = registry.get(id)
  if (!entry || !term) return false
  const decorations = searchDecorations()
  return direction === 'next' ? entry.search.findNext(term, decorations) : entry.search.findPrevious(term, decorations)
}

export function clearSearchSession(id: string): void {
  const entry = registry.get(id)
  if (!entry) return
  entry.search.clearDecorations()
  entry.search.clearActiveDecoration()
  entry.term.clearSelection()
}

export function focusSession(id: string): void {
  registry.get(id)?.term.focus()
}

// Below this, FitAddon computes a degenerate 0-2 col/row terminal -- fitting
// (and pushing that size to the backend PTY) at that point corrupts the
// terminal's rendering in a way a later, normal-sized fit doesn't recover
// from. A pane this small isn't usable anyway, so skip the fit entirely
// rather than let it run with near-zero dimensions.
const MIN_FIT_SIZE = 50

function fitNow(entry: TerminalEntry): void {
  if (!entry.attached) return
  const { clientWidth, clientHeight } = entry.attached
  if (clientWidth < MIN_FIT_SIZE || clientHeight < MIN_FIT_SIZE) return
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

export function setScrollback(lines: number): void {
  currentScrollback = Math.min(MAX_SCROLLBACK, Math.max(MIN_SCROLLBACK, lines))
  registry.forEach((entry) => {
    entry.term.options.scrollback = currentScrollback
  })
}

/** Re-applies the current theme to every live terminal -- called by the
 * settings bridge whenever the user switches appearance.theme. */
export function applyTerminalTheme(): void {
  const theme = readTerminalTheme()
  registry.forEach((entry) => {
    entry.term.options.theme = theme
  })
}

export function disposeSession(id: string): void {
  const entry = registry.get(id)
  if (!entry) return
  entry.unsubs.forEach((unsub) => unsub())
  entry.term.dispose()
  void closeSession(id)
  registry.delete(id)
  useSessionStore.getState().remove(id)
  useHostKeyPromptStore.getState().clearPrompt(id)
}
