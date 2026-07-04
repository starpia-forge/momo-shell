import { create } from 'zustand'
import {
  listHosts,
  saveHost as apiSaveHost,
  deleteHost as apiDeleteHost,
  listLabels as apiListLabels,
  type Host,
  type HostInput,
} from '../../../shared/api/host'

interface HostStore {
  hosts: Record<string, Host>
  labels: string[]
  loaded: boolean
  load: () => Promise<void>
  save: (input: HostInput) => Promise<Host>
  remove: (id: string) => Promise<void>
}

export const useHostStore = create<HostStore>((set) => ({
  hosts: {},
  labels: [],
  loaded: false,
  load: async () => {
    const [hosts, labels] = await Promise.all([listHosts(), apiListLabels()])
    set({
      hosts: Object.fromEntries(hosts.map((h) => [h.id, h])),
      labels,
      loaded: true,
    })
  },
  save: async (input) => {
    const host = await apiSaveHost(input)
    const labels = await apiListLabels()
    set((s) => ({ hosts: { ...s.hosts, [host.id]: host }, labels }))
    return host
  },
  remove: async (id) => {
    await apiDeleteHost(id)
    set((s) => {
      const rest = { ...s.hosts }
      delete rest[id]
      return { hosts: rest }
    })
  },
}))
