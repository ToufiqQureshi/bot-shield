export default function Changelog() {
  return (
    <div className="min-h-screen px-6 py-16" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="max-w-2xl mx-auto space-y-7">
        <a href="/landing" className="font-bold text-sm">hakaishield</a>
        <h1 className="text-3xl font-semibold">Pilot update</h1>
        <div className="card p-6 space-y-3">
          <p className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>2026-09-24</p>
          <h2 className="font-semibold">First-client pilot preparation</h2>
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            The Go proxy and dashboard are prepared for one managed domain, with shadow-mode
            review, request evidence, TLS fingerprinting and a safe deployment checklist.
            Live traffic results will be measured after the first client domain is connected.
          </p>
        </div>
      </div>
    </div>
  );
}
