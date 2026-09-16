// Real API client for the scoring engine's stats endpoint
// (proxy/stats.go): GET /api/v1/dashboard/stats.
export interface DashboardStatsData {
  total_requests: number;
  passed: number;
  challenged: number;
  blocked: number;
  // In shadow mode the counts describe decisions that were recorded
  // but never acted on, so nothing may be shown without them.
  mode: "enforce" | "shadow";
  enforcing: boolean;
}

// Overridable so a deployed dashboard can point at a botshield
// instance that isn't on localhost; defaults to local dev.
const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";

export async function fetchStats(): Promise<DashboardStatsData> {
  const res = await fetch(`${API_BASE_URL}/api/v1/dashboard/stats`);
  if (!res.ok) {
    throw new Error(`stats request failed with status ${res.status}`);
  }
  return res.json();
}
