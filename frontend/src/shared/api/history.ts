import { ClearHistory, DeleteHistoryEntry, ListHistory } from '../../../wailsjs/go/wails/HistoryService'

export interface HistoryEntry {
  id: number
  hostId?: string
  command: string
  executedAt: number
}

/** "" = every session, "local" = local shells only, otherwise a host ID. */
export type HistoryScope = '' | 'local' | string

export interface HistoryQuery {
  hostId?: HistoryScope
  search?: string
  limit?: number
  offset?: number
}

export async function listHistory(query: HistoryQuery = {}): Promise<HistoryEntry[]> {
  return ListHistory({
    hostId: query.hostId ?? '',
    search: query.search ?? '',
    limit: query.limit ?? 0,
    offset: query.offset ?? 0,
  })
}

export async function deleteHistoryEntry(id: number): Promise<void> {
  await DeleteHistoryEntry(id)
}

export async function clearHistory(scope: HistoryScope = ''): Promise<void> {
  await ClearHistory(scope)
}
