export default function Docs() {
  return (
    <div className="min-h-screen px-6 py-16" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="max-w-3xl mx-auto space-y-8">
        <a href="/landing" className="font-bold text-sm">hakaishield</a>
        <div>
          <p className="text-xs font-mono mb-3" style={{ color: 'var(--text-muted)' }}>// pilot guide</p>
          <h1 className="text-3xl font-semibold mb-3">How the pilot works</h1>
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            HakaiShield terminates TLS at the pilot proxy, evaluates each request, and forwards
            allowed traffic to your origin. The dashboard shows request totals and recent evidence.
          </p>
        </div>
        <ol className="space-y-4">
          <li className="card p-5"><strong>1. Confirm your domain and origin.</strong><p className="text-sm mt-2" style={{ color: 'var(--text-secondary)' }}>The operator verifies ownership and checks that your origin is reachable without creating a proxy loop.</p></li>
          <li className="card p-5"><strong>2. Configure DNS and TLS.</strong><p className="text-sm mt-2" style={{ color: 'var(--text-secondary)' }}>The operator provides the server address and installs a certificate for the exact hostname.</p></li>
          <li className="card p-5"><strong>3. Review shadow traffic.</strong><p className="text-sm mt-2" style={{ color: 'var(--text-secondary)' }}>Requests still reach your origin while we review decisions and legitimate-user false positives.</p></li>
          <li className="card p-5"><strong>4. Approve enforcement.</strong><p className="text-sm mt-2" style={{ color: 'var(--text-secondary)' }}>The operator enables blocking only after a successful review and keeps a rollback path.</p></li>
        </ol>
        <a href="/contact" className="btn-primary inline-block px-5 py-2.5 text-sm">Contact the operator</a>
      </div>
    </div>
  );
}
