// Thin fetch client for the hakaishield backend. Every JWT-authenticated
// endpoint returns {"success": boolean, "data"|"message"|"error"} (see
// dashboard/BACKEND_WIRING_DOCS.md and backend/pkg/api/response.go) —
// this file is the one place that unwraps that envelope, so a page
// component only ever deals with plain data or a thrown ApiError.

const BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api/v1';
const TOKEN_KEY = 'hakaishield-token';

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export function getToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setToken(token: string) {
  try {
    localStorage.setItem(TOKEN_KEY, token);
  } catch {
    // Private browsing / blocked storage: the session just won't
    // survive a reload. Nothing here can recover from that, and the
    // caller's next authenticated request will fail loudly with a 401
    // instead of silently pretending to be signed in.
  }
}

export function clearToken() {
  try {
    localStorage.removeItem(TOKEN_KEY);
  } catch {
    // See setToken.
  }
}

export function isSignedIn(): boolean {
  return !!getToken();
}

interface Envelope<T> {
  success: boolean;
  data?: T;
  message?: string;
  error?: string;
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken();
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

  if (res.status === 401) {
    clearToken();
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

// ---- Auth ----

export interface AuthUser {
  id: string;
  email: string;
  name: string;
  company?: string;
  onboardingComplete: boolean;
}

export async function signup(name: string, email: string, password: string, company?: string) {
  return request<{ userId: string; email: string }>('/auth/signup', {
    method: 'POST',
    body: JSON.stringify({ name, email, password, company }),
  });
}

export async function signin(email: string, password: string) {
  const data = await request<{ token: string; user: AuthUser }>('/auth/signin', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
  setToken(data.token);
  return data.user;
}

export function signout() {
  clearToken();
}

export async function me() {
  return request<AuthUser>('/auth/me');
}

export async function completeOnboarding() {
  return request<string>('/onboarding/complete', { method: 'POST' });
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
