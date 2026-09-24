export default function Privacy() {
  return (
    <div className="min-h-screen px-6 py-16" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="max-w-xl mx-auto space-y-6">
        <a href="/landing" className="font-bold text-sm">hakaishield</a>
        <h1 className="text-3xl font-semibold">Pilot privacy notice</h1>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          The pilot operator will provide a reviewed privacy notice and data-processing terms
          before the client routes visitor traffic. No public policy is published here yet.
        </p>
        <a href="/contact" className="btn-secondary inline-block px-5 py-2.5 text-sm">Contact operator</a>
      </div>
    </div>
  );
}
