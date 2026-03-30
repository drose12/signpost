// web/src/types.ts

export interface Domain {
  id: number;
  name: string;
  dkim_selector: string;
  dkim_key_path?: string;
  dkim_public_dns?: string;
  dkim_created_at?: string;
  spf_record?: string;
  dmarc_record?: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface RelayConfig {
  id: number;
  domain_id: number;
  method: string; // gmail, isp, direct, custom
  host?: string;
  port: number;
  username?: string;
  starttls: boolean;
  created_at: string;
  updated_at: string;
}

export interface MailLogEntry {
  id: number;
  timestamp: string;
  from_addr: string;
  to_addr: string;
  domain_id?: number;
  subject?: string;
  status: string; // sent, failed, deferred
  relay_host?: string;
  error?: string;
  dkim_signed: boolean;
}

export interface DNSRecord {
  type: string;
  name: string;
  value: string;
  description: string;
}

export interface StatusResponse {
  domain_count: number;
  tls_mode: string;
  tls_cert_expiry?: string;
  schema_version: number;
  maddy_status: string;
}

export interface DKIMGenerateResponse {
  dns_record_name: string;
  dns_record_value: string;
  selector: string;
  key_path: string;
}

export interface TestSendResponse {
  status: string;
  message?: string;
  error?: string;
}

export interface RelayTestResponse {
  status: string;
  message?: string;
  error?: string;
}

export interface DNSCheckRecord {
  type: string;
  name: string;
  purpose: string;
  current: string | null;
  recommended: string;
  status: 'ok' | 'missing' | 'update' | 'conflict';
  message: string;
}

export interface DNSCheckResponse {
  records: DNSCheckRecord[];
}
