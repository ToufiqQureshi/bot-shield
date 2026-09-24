import { ArrowRight } from 'lucide-react';

export default function Pricing() {
  return (
    <div className="min-h-screen px-6 py-16" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="max-w-3xl mx-auto space-y-8">
        <a href="/landing" className="font-bold text-sm">hakaishield</a>
        <div>
          <p className="text-xs font-mono mb-3" style={{ color: 'var(--text-muted)' }}>// pilot pricing</p>
          <h1 className="text-4xl font-semibold mb-4">Pricing for your traffic</h1>
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            We are onboarding the first client through a managed pilot. Pricing and limits are agreed
            after reviewing your domain, origin, traffic and support needs. Online checkout is not available yet.
          </p>
        </div>
        <div className="card p-6 space-y-4">
          <h2 className="font-semibold">Pilot setup</h2>
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            Domain and TLS setup, shadow mode review, evidence walkthrough, and a controlled enforcement decision.
          </p>
          <a href="/contact" className="btn-primary inline-flex items-center gap-2 px-5 py-2.5 text-sm">Discuss a pilot <ArrowRight size={14} /></a>
        </div>
      </div>
    </div>
  );
}
