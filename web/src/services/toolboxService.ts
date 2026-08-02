import { getAuthenticatedClient, request } from '@/core/http/client'
import type {
  DnsLookupRequest,
  DnsLookupResponse,
  PortScanRequest,
  PortScanResponse,
  SslCheckRequest,
  SslCheckResponse,
  WhoisRequest,
  WhoisResponse,
} from '@/types/toolbox'

// v1 endpoints wrap payloads in a { data } envelope; unwrap it here.
interface V1Envelope<T> {
  data: T
}

// Toolbox one-shot network tools. Each call accepts an optional
// AbortSignal so the UI can cancel slow external lookups.

export const dnsLookup = async (
  payload: DnsLookupRequest,
  signal?: AbortSignal,
): Promise<DnsLookupResponse> => {
  const envelope = await request<V1Envelope<DnsLookupResponse>>(
    getAuthenticatedClient(),
    'v1/toolbox/dns',
    { method: 'POST', json: payload, signal },
  )
  return envelope.data
}

export const portScan = async (
  payload: PortScanRequest,
  signal?: AbortSignal,
): Promise<PortScanResponse> => {
  const envelope = await request<V1Envelope<PortScanResponse>>(
    getAuthenticatedClient(),
    'v1/toolbox/port-scan',
    { method: 'POST', json: payload, signal },
  )
  return envelope.data
}

export const sslCheck = async (
  payload: SslCheckRequest,
  signal?: AbortSignal,
): Promise<SslCheckResponse> => {
  const envelope = await request<V1Envelope<SslCheckResponse>>(
    getAuthenticatedClient(),
    'v1/toolbox/ssl-check',
    { method: 'POST', json: payload, signal },
  )
  return envelope.data
}

export const whoisLookup = async (
  payload: WhoisRequest,
  signal?: AbortSignal,
): Promise<WhoisResponse> => {
  const envelope = await request<V1Envelope<WhoisResponse>>(
    getAuthenticatedClient(),
    'v1/toolbox/whois',
    { method: 'POST', json: payload, signal },
  )
  return envelope.data
}
