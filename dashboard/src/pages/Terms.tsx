export default function Terms() {
  return (
    <div className="min-h-screen px-6 py-16" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="max-w-xl mx-auto space-y-6">
        <a href="/landing" className="font-bold text-sm">hakaishield</a>
        <h1 className="text-3xl font-semibold">Pilot terms</h1>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Public terms are not published yet. The pilot operator will provide the agreement
          for review before any client traffic is routed through HakaiShield.
        </p>
        <a href="/contact" className="btn-secondary inline-block px-5 py-2.5 text-sm">Contact operator</a>
      </div>
    </div>
  );
}
