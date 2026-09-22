import { useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { Globe, Webhook, Plus, AlertCircle, X } from 'lucide-react';
import { addDomain, ApiError } from '../lib/api';
import type { LayoutContext } from '../components/Layout';

export default function DomainsSiem() {
  const { domains, domainsLoading, onDomainAdded } = useOutletContext<LayoutContext>();
  const [showAdd, setShowAdd] = useState(false);
  const [newDomain, setNewDomain] = useState('');
  const [newOrigin, setNewOrigin] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const created = await addDomain(newDomain.trim(), newOrigin.trim());
      onDomainAdded(created);
      setShowAdd(false);
      setNewDomain('');
      setNewOrigin('');
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Could not add domain.');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="space-y-6 animate-in">
      {/* Header */}
      <div>
        <h1 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>Domains & SIEM</h1>
        <p className="text-sm" style={{ color: 'var(--text-muted)' }}>Manage protected domains and data export integrations</p>
      </div>

      {/* Protected Domains */}
      <div className="card overflow-hidden">
        <div className="px-5 py-4 border-b flex items-center justify-between" style={{ borderColor: 'var(--border-primary)' }}>
          <div className="flex items-center gap-2">
            <Globe size={16} style={{ color: 'var(--text-muted)' }} />
            <div>
              <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Protected Domains</h2>
              <p className="text-xs" style={{ color: 'var(--text-muted)' }}>Sites routed through HakaiShield edge</p>
            </div>
          </div>
          <button onClick={() => setShowAdd(true)} className="btn-primary text-xs flex items-center gap-1.5">
            <Plus size={12} /> Add Domain
          </button>
        </div>

        {showAdd && (
          <form onSubmit={handleAdd} className="px-5 py-4 border-b space-y-3" style={{ borderColor: 'var(--border-primary)', background: 'var(--bg-secondary)' }}>
            <div className="flex items-center justify-between">
              <p className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>Add a domain</p>
              <button type="button" onClick={() => setShowAdd(false)} style={{ color: 'var(--text-muted)' }}><X size={14} /></button>
            </div>
            <div className="grid sm:grid-cols-2 gap-3">
              <input
                required
                placeholder="app.example.com"
                value={newDomain}
                onChange={(e) => setNewDomain(e.target.value)}
                className="px-3 py-2 rounded text-xs"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
              />
              <input
                required
                placeholder="origin, e.g. 10.0.1.50:8080"
                value={newOrigin}
                onChange={(e) => setNewOrigin(e.target.value)}
                className="px-3 py-2 rounded text-xs"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
              />
            </div>
            {error && <p className="text-xs" style={{ color: 'var(--accent-red, #ef4444)' }}>{error}</p>}
            <button type="submit" disabled={submitting} className="btn-primary text-xs px-4 py-2 disabled:opacity-60">
              {submitting ? 'Adding…' : 'Add domain'}
            </button>
          </form>
        )}

        <div className="overflow-x-auto">
          {domainsLoading ? (
            <p className="px-5 py-6 text-xs" style={{ color: 'var(--text-muted)' }}>Loading domains…</p>
          ) : domains.length === 0 ? (
            <p className="px-5 py-6 text-xs" style={{ color: 'var(--text-muted)' }}>No domains yet — add one to start routing traffic through hakaishield.</p>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>Domain</th>
                  <th>Origin</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {domains.map(domain => (
                  <tr key={domain.id}>
                    <td>
                      <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{domain.domain}</span>
                    </td>
                    <td className="font-mono text-xs">{domain.origin}</td>
                    <td>
                      <span className={`badge ${domain.status === 'active' ? 'badge-green' : 'badge-yellow'}`}>
                        {domain.status === 'pending_verification' ? 'pending' : domain.status}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {/* SIEM Integrations — not built yet */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Webhook size={16} style={{ color: 'var(--text-muted)' }} />
          <div>
            <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>SIEM Integrations</h2>
            <p className="text-xs" style={{ color: 'var(--text-muted)' }}>Export logs and events to your security stack</p>
          </div>
        </div>
        <div className="card p-4 flex items-center gap-3">
          <AlertCircle size={16} className="text-yellow-400 shrink-0" />
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
            SIEM export (Datadog, Splunk, S3, webhooks) isn't built yet — this section is a placeholder until a real
            integration ships, rather than a working toggle that would connect to nothing.
          </p>
        </div>
      </div>
    </div>
  );
}
