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

// A response we can't fully understand is treated as a failure, not
// as defaults. A missing `enforcing` would read as false and make the
// dashboard announce "nothing is being blocked" while the proxy is
// enforcing — the same lie the shadow banner exists to prevent, only
// inverted.
function parseStats(body: unknown): DashboardStatsData {
  const b = body as Record<string, unknown>;
  if (!b || typeof b !== "object") {
    throw new Error("stats response was not an object");
  }
  for (const field of ["total_requests", "passed", "challenged", "blocked"]) {
    if (typeof b[field] !== "number" || !Number.isFinite(b[field] as number)) {
      throw new Error(`stats response field ${field} is not a number`);
    }
  }
  if (typeof b.enforcing !== "boolean") {
    throw new Error("stats response is missing the enforcing flag");
  }
  if (b.mode !== "enforce" && b.mode !== "shadow") {
    throw new Error(`stats response has unknown mode ${String(b.mode)}`);
  }
  return b as unknown as DashboardStatsData;
}

export async function fetchStats(): Promise<DashboardStatsData> {
  const res = await fetch(`${API_BASE_URL}/api/v1/dashboard/stats`);
  if (!res.ok) {
    throw new Error(`stats request failed with status ${res.status}`);
  }
  return parseStats(await res.json());
}
