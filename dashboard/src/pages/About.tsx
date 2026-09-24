export default function About() {
  return (
    <div className="min-h-screen px-6 py-16" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="max-w-2xl mx-auto space-y-7">
        <a href="/landing" className="font-bold text-sm">hakaishield</a>
        <h1 className="text-3xl font-semibold">About HakaiShield</h1>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          HakaiShield is an inline traffic intelligence service. It combines TLS fingerprints,
          request signals and observed traffic patterns to explain decisions about automated requests.
        </p>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          The first client rollout is a managed pilot. We measure real traffic, latency and false
          positives before claiming a detection rate or enabling enforcement.
        </p>
        <a href="/contact" className="btn-primary inline-block px-5 py-2.5 text-sm">Discuss a pilot</a>
      </div>
    </div>
  );
}
