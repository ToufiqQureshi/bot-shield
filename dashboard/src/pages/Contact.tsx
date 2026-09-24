import { Mail } from 'lucide-react';

const pilotContact = (import.meta.env.VITE_PILOT_CONTACT_EMAIL || '').trim();

export default function Contact() {
  return (
    <div className="min-h-screen px-6 py-16" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="max-w-2xl mx-auto space-y-8">
        <a href="/landing" className="font-bold text-sm">hakaishield</a>
        <div>
          <p className="text-xs font-mono mb-3" style={{ color: 'var(--text-muted)' }}>// pilot contact</p>
          <h1 className="text-3xl font-semibold mb-3">Talk to the pilot operator</h1>
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            Share the domain you control, your origin URL and the traffic you want to protect.
            We will confirm DNS ownership and test the site in shadow mode before routing visitors.
          </p>
        </div>
        <div className="card p-6 space-y-4">
          <Mail size={22} className="text-blue-400" />
          {pilotContact ? (
            <>
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>Email the operator directly:</p>
              <a className="btn-primary inline-block px-5 py-2.5 text-sm" href={`mailto:${pilotContact}?subject=HakaiShield%20pilot%20setup`}>
                {pilotContact}
              </a>
            </>
          ) : (
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              Contact your pilot operator using your agreed channel. Online requests are not enabled yet.
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
