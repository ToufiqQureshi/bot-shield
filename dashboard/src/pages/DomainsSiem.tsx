import { useOutletContext } from 'react-router-dom';
import { AlertCircle, Globe } from 'lucide-react';
import { domainState } from '../lib/domainStatus';
import type { LayoutContext } from '../components/Layout';

export default function DomainsSiem() {
  const { domains, domainsLoading } = useOutletContext<LayoutContext>();

  return (
    <div className="space-y-6 animate-in">
      <div>
        <h1 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>Pilot domains</h1>
        <p className="text-sm" style={{ color: 'var(--text-muted)' }}>See which sites have completed setup.</p>
      </div>

      <div className="card p-5 flex items-start gap-3">
        <AlertCircle size={18} className="text-yellow-400 shrink-0 mt-0.5" />
        <div className="space-y-2">
          <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>Domain setup is managed during the pilot</p>
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
            Contact the pilot operator with your domain and origin URL. We verify ownership, configure DNS and TLS,
            and test routing before marking it protected. A domain shown as pending is not protected yet.
          </p>
          <a href="/contact" className="text-xs underline" style={{ color: 'var(--accent-blue)' }}>Request domain setup</a>
        </div>
      </div>

      <div className="card overflow-hidden">
        <div className="px-5 py-4 border-b flex items-center gap-2" style={{ borderColor: 'var(--border-primary)' }}>
          <Globe size={16} style={{ color: 'var(--text-muted)' }} />
          <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Your domains</h2>
        </div>
        {domainsLoading ? (
          <p className="px-5 py-6 text-xs" style={{ color: 'var(--text-muted)' }}>Loading domains…</p>
        ) : domains.length === 0 ? (
          <p className="px-5 py-6 text-xs" style={{ color: 'var(--text-muted)' }}>
            No domain is connected to this account. Request pilot setup to get started.
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="data-table">
              <thead><tr><th>Domain</th><th>Origin</th><th>Status</th></tr></thead>
              <tbody>
                {domains.map((domain) => {
                  const state = domainState(domain.status);
                  return (
                    <tr key={domain.id}>
                      <td className="text-sm font-medium">{domain.domain}</td>
                      <td className="font-mono text-xs">{domain.origin}</td>
                      <td>
                        <span className={`badge ${state.protected ? 'badge-green' : 'badge-yellow'}`}>{state.label}</span>
                        <p className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>{state.detail}</p>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
