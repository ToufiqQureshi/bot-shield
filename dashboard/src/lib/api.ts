// Thin fetch client for the hakaishield backend's domains/rules/settings
// API. Every endpoint returns {"success": boolean, "data"|"message"|"error"}
// (see backend/pkg/api/response.go) — this file is the one place that
// unwraps that envelope, so a page component only ever deals with plain
// data or a thrown ApiError.
//
// Auth itself (signup, signin, sign-out, password reset, email
// verification) is not here — it goes straight to Supabase via
// supabaseClient.ts. This file only attaches whatever session token
// Supabase already issued when calling the Go backend.

import { supabase } from './supabaseClient';

const BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api/v1';

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export async function isSignedIn(): Promise<boolean> {
  const { data } = await supabase.auth.getSession();
  return !!data.session;
}

interface Envelope<T> {
  success: boolean;
  data?: T;
  message?: string;
  error?: string;
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const { data: sessionData } = await supabase.auth.getSession();
  const token = sessionData.session?.access_token;

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> | undefined),
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  let res: Response;
  try {
    res = await fetch(`${BASE_URL}${path}`, { ...options, headers });
  } catch {
    throw new ApiError(0, 'Could not reach the server. Check your connection and try again.');
  }

  let body: Envelope<T> | null = null;
  try {
    body = await res.json();
  } catch {
    // A non-JSON response (proxy error page, empty 204, etc.) is
    // still handled by the status-code branch below rather than
    // treated as a parse failure the caller has to know about.
  }

  if (!res.ok || body?.success === false) {
    throw new ApiError(res.status, body?.error || body?.message || `Request failed (${res.status})`);
  }

  return (body?.data ?? body?.message ?? null) as T;
}

// ---- Domains ----

export interface Domain {
  id: string;
  domain: string;
  origin: string;
  name: string;
  status: string;
}

export async function listDomains() {
  return request<Domain[]>('/domains');
}

export async function addDomain(domain: string, origin: string) {
  return request<Domain>('/domains', {
    method: 'POST',
    body: JSON.stringify({ domain, origin }),
  });
}

// ---- Mitigation Rules ----

export interface ManagedRule {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
}

export interface RuleCondition {
  field: string;
  operator: string;
  value: string;
}

export interface CustomRule {
  id: string;
  name: string;
  conditions: RuleCondition[];
  action: string;
  enabled: boolean;
  createdAt: string;
}

export interface RulesResponse {
  managedRules: ManagedRule[];
  customRules: CustomRule[];
  exceptions: unknown[];
}

export async function listRules() {
  return request<RulesResponse>('/rules');
}

export async function createRule(name: string, conditions: RuleCondition[], action: string) {
  return request<CustomRule>('/rules/custom', {
    method: 'POST',
    body: JSON.stringify({ name, conditions, action }),
  });
}

export async function toggleRule(id: string, enabled: boolean) {
  return request<string>(`/rules/${id}/toggle`, {
    method: 'PUT',
    body: JSON.stringify({ enabled }),
  });
}

// ---- Protection Settings ----

export interface ProtectionSettings {
  blockThreshold: number;
  challengeThreshold: number;
  challengeType: string;
  honeypotEnabled: boolean;
}

export async function getProtectionSettings() {
  return request<ProtectionSettings>('/settings/protection');
}

export async function updateProtectionSettings(settings: ProtectionSettings) {
  return request<string>('/settings/protection', {
    method: 'PUT',
    body: JSON.stringify(settings),
  });
}

// ---- Dashboard ----

export interface DashboardStats {
  total_requests: number;
  passed: number;
  challenged: number;
  blocked: number;
  deceived: number;
  mode: string;
  enforcing: boolean;
}

// getStats talks to the pre-existing, unauthenticated
// /api/v1/dashboard/stats?tenant=<id> endpoint (backend/pkg/api/handlers.go)
// rather than going through request()'s JWT envelope — that endpoint
// predates this dashboard and has its own response shape.
export async function getStats(tenantId: string): Promise<DashboardStats> {
  const res = await fetch(`${BASE_URL.replace(/\/api\/v1$/, '')}/api/v1/dashboard/stats?tenant=${encodeURIComponent(tenantId)}`);
  if (!res.ok) {
    throw new ApiError(res.status, `Could not load stats (${res.status})`);
  }
  return res.json();
}

export interface TopOffender {
  ja4: string;
  hits: number;
  blocked: number;
}

export async function getTopOffenders() {
  return request<TopOffender[]>('/dashboard/top-offenders');
}

export interface EvidenceEntry {
  time: string;
  ja4: string;
  signals: string[];
  score: number;
  decision: string;
  enforced: boolean;
}

export async function getEvidenceLogs() {
  return request<EvidenceEntry[]>('/dashboard/evidence-logs');
}
