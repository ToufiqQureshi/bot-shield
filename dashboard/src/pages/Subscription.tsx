export default function Subscription() {
  return (
    <div className="min-h-screen px-6 py-16" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="max-w-2xl mx-auto space-y-6">
        <a href="/" className="font-bold text-sm">hakaishield</a>
        <h1 className="text-3xl font-semibold">Subscription</h1>
        <div className="card p-6 space-y-4">
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            Billing and usage metering are not available in this dashboard during the managed pilot.
            Your operator will confirm the agreed price and limits directly.
          </p>
          <a href="/contact" className="btn-secondary inline-block px-5 py-2.5 text-sm">Contact operator</a>
        </div>
      </div>
    </div>
  );
}
