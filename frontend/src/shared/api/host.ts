import {
  ListHosts,
  GetHost,
  SaveHost,
  DeleteHost,
  SetHostSecret,
  TestConnection,
  ListLabels,
} from '../../../wailsjs/go/wails/HostService'

export type AuthType = 'password' | 'privateKey' | 'agent'

export interface Host {
  id: string
  name: string
  address: string
  port: number
  labels: string[]
  username: string
  authType: AuthType
  keyPath?: string
  source: string
  createdAt: number
  updatedAt: number
  lastConnectedAt?: number
}

export interface HostInput {
  id?: string
  name: string
  address: string
  port: number
  labels: string[]
  username: string
  authType: AuthType
  keyPath?: string
}

export interface TestResult {
  stage: 'tcp' | 'handshake' | 'auth'
  ok: boolean
  message?: string
}

// asHost/asTestResult narrow the wails-generated DTOs' plain `string` fields
// (Go's string-typed JSON tags don't carry Go's enum-like const values into
// the generated TS) to this module's literal union types.
function asHost(dto: Awaited<ReturnType<typeof GetHost>>): Host {
  return { ...dto, authType: dto.authType as AuthType }
}

function asTestResult(dto: Awaited<ReturnType<typeof TestConnection>>): TestResult {
  return { ...dto, stage: dto.stage as TestResult['stage'] }
}

export async function listHosts(): Promise<Host[]> {
  const hosts = await ListHosts()
  return (hosts ?? []).map(asHost)
}

export async function getHost(id: string): Promise<Host> {
  return asHost(await GetHost(id))
}

export async function saveHost(input: HostInput): Promise<Host> {
  return asHost(
    await SaveHost({
      id: input.id ?? '',
      name: input.name,
      address: input.address,
      port: input.port,
      labels: input.labels,
      username: input.username,
      authType: input.authType,
      keyPath: input.keyPath ?? '',
    })
  )
}

export async function deleteHost(id: string): Promise<void> {
  await DeleteHost(id)
}

export async function setHostSecret(id: string, secret: string): Promise<void> {
  await SetHostSecret(id, secret)
}

export async function testConnection(id: string): Promise<TestResult> {
  return asTestResult(await TestConnection(id))
}

export async function listLabels(): Promise<string[]> {
  const labels = await ListLabels()
  return labels ?? []
}
