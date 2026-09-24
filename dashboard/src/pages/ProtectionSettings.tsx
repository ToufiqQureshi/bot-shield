import { AlertCircle, Shield } from 'lucide-react';

export default function ProtectionSettings() {
  return (
    <div className="space-y-6 animate-in">
      <div>
        <h1 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>Protection settings</h1>
        <p className="text-sm" style={{ color: 'var(--text-muted)' }}>Pilot policy and rollout controls.</p>
      </div>
      <div className="card p-6 space-y-4 max-w-2xl">
        <Shield size={22} className="text-blue-400" />
        <h2 className="text-base font-semibold">Managed pilot policy</h2>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Your operator starts protection in shadow mode, reviews the evidence with you,
          and enables enforcement only after the impact is approved. Policy changes
          are applied and verified by the operator during this pilot.
        </p>
        <div className="flex items-start gap-2 rounded p-3" style={{ background: 'var(--bg-tertiary)' }}>
          <AlertCircle size={16} className="text-yellow-400 shrink-0 mt-0.5" />
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
            Self-service controls are unavailable during this pilot. The operator
            applies each approved change and confirms its effect on real traffic.
          </p>
        </div>
        <a href="/contact" className="btn-secondary inline-block px-5 py-2.5 text-sm">Request a policy change</a>
      </div>
    </div>
  );
}
