import { getAuthenticatedClient, request } from '@/core/http/client'
import type {
  DiskUsage,
  Host,
  HostMetricSample,
  RegisterHostResult,
  HostCredentialResult,
  MonitorSummary,
} from '@/types'

/**
 * Hosts feed — backed by the real v1 API. v1 endpoints
 * wrap payloads in a `{ data }` envelope and use snake_case DTOs; this service
 * unwraps and maps them to the camelCase `Host` view models. Frontend-only: no
 * backend change.
 */

const successMsg = (m: string) => ({ headers: { 'x-success-message': m } })

// ---- raw v1 DTO shapes (snake_case) ----
interface DiskDTO {
  mount?: string
  used_pct?: number
}
interface HostDTO {
  id?: string
  name?: string
  os?: string | null
  agent_version?: string | null
  online?: boolean
  last_seen_at?: string | null
  last_cpu_pct?: number | null
  last_mem_pct?: number | null
  last_disk_pct?: number | null
  last_net_in?: number | null
  last_net_out?: number | null
  last_disks?: DiskDTO[] | null
  created_at?: string
  updated_at?: string
}
interface HostMetricSampleDTO {
  sampled_at?: string
  cpu_pct?: number
  mem_pct?: number
  net_in?: number
  net_out?: number
  disks?: DiskDTO[] | null
}
interface RegisterHostDTO {
  host: HostDTO
  credential: string
  prefix: string
}
interface CredentialDTO {
  credential: string
  prefix: string
}

function mapDisks(d?: DiskDTO[] | null): DiskUsage[] {
  return (d ?? []).map((x) => ({ mount: x.mount ?? '', usedPct: x.used_pct ?? 0 }))
}

export function mapHost(dto: HostDTO): Host {
  return {
    id: dto.id ?? '',
    name: dto.name ?? '',
    os: dto.os ?? null,
    agentVersion: dto.agent_version ?? null,
    online: dto.online ?? false,
    lastSeenAt: dto.last_seen_at ?? null,
    lastCpuPct: dto.last_cpu_pct ?? null,
    lastMemPct: dto.last_mem_pct ?? null,
    lastDiskPct: dto.last_disk_pct ?? null,
    lastNetIn: dto.last_net_in ?? null,
    lastNetOut: dto.last_net_out ?? null,
    lastDisks: mapDisks(dto.last_disks),
    createdAt: dto.created_at ?? '',
    updatedAt: dto.updated_at ?? '',
  }
}

function mapSample(dto: HostMetricSampleDTO): HostMetricSample {
  return {
    sampledAt: dto.sampled_at ?? '',
    cpuPct: dto.cpu_pct ?? 0,
    memPct: dto.mem_pct ?? 0,
    netIn: dto.net_in ?? 0,
    netOut: dto.net_out ?? 0,
    disks: mapDisks(dto.disks),
  }
}

const client = () => getAuthenticatedClient()

export async function listHosts(): Promise<Host[]> {
  const res = await request<{ data: HostDTO[] }>(client(), 'v1/hosts')
  return (res?.data ?? []).map(mapHost)
}

export async function getHost(id: string): Promise<Host> {
  const res = await request<{ data: HostDTO }>(client(), `v1/hosts/${id}`)
  return mapHost(res.data)
}

export async function registerHost(input: { name: string }): Promise<RegisterHostResult> {
  const res = await request<{ data: RegisterHostDTO }>(client(), 'v1/hosts', {
    method: 'POST',
    json: input,
    ...successMsg('Host registered'),
  })
  return { host: mapHost(res.data.host), credential: res.data.credential, prefix: res.data.prefix }
}

export async function rotateCredential(id: string): Promise<HostCredentialResult> {
  const res = await request<{ data: CredentialDTO }>(client(), `v1/hosts/${id}/credential/rotate`, {
    method: 'POST',
    ...successMsg('Credential rotated'),
  })
  return { credential: res.data.credential, prefix: res.data.prefix }
}

export async function revokeCredential(id: string): Promise<void> {
  await request<void>(client(), `v1/hosts/${id}/credential/revoke`, {
    method: 'POST',
    ...successMsg('Credential revoked'),
  })
}

export async function deleteHost(id: string): Promise<void> {
  await request<void>(client(), `v1/hosts/${id}`, {
    method: 'DELETE',
    ...successMsg('Host deleted'),
  })
}

export async function getHostMetrics(
  id: string,
  from?: Date,
  to?: Date,
): Promise<HostMetricSample[]> {
  const searchParams: Record<string, string> = {}
  if (from) searchParams.from = from.toISOString()
  if (to) searchParams.to = to.toISOString()
  const res = await request<{ data: HostMetricSampleDTO[] }>(client(), `v1/hosts/${id}/metrics`, {
    searchParams: Object.keys(searchParams).length > 0 ? searchParams : undefined,
  })
  return (res?.data ?? []).map(mapSample)
}

interface MonitorDTO {
  id?: string
  name?: string
  type?: string
  status?: string
  last_checked_at?: string | null
  host_id?: string | null
}

/**
 * List monitors via the v1 endpoint — the only monitor read that includes
 * `host_id`. Used to derive per-host service counts and the hosted-services /
 * host-panel views. Kept minimal (a projection); the root resources endpoint
 * does not carry the host link.
 */
export async function listMonitors(): Promise<MonitorSummary[]> {
  const res = await request<{ data: MonitorDTO[] }>(client(), 'v1/monitors')
  return (res?.data ?? []).map((m) => ({
    id: m.id ?? '',
    name: m.name ?? '',
    type: m.type ?? '',
    status: m.status ?? '',
    lastCheckedAt: m.last_checked_at ?? null,
    hostId: m.host_id ?? null,
  }))
}

export async function linkMonitorToHost(monitorId: string, hostId: string): Promise<void> {
  await request<{ data: unknown }>(client(), `v1/monitors/${monitorId}/host`, {
    method: 'POST',
    json: { host_id: hostId },
    ...successMsg('Monitor linked to host'),
  })
}

export async function unlinkMonitorFromHost(monitorId: string): Promise<void> {
  await request<void>(client(), `v1/monitors/${monitorId}/host`, {
    method: 'DELETE',
    ...successMsg('Monitor unlinked from host'),
  })
}
