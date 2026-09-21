import { useState } from 'react';
import { Globe, Webhook, Plus, CheckCircle2, XCircle, AlertCircle, ExternalLink } from 'lucide-react';
import { domains, siemIntegrations } from '../data/mockData';

export default function DomainsSiem() {
  const [domainList] = useState(domains);
  const [integrations, setIntegrations] = useState(siemIntegrations);

  const toggleIntegration = (id: string) => {
    setIntegrations(integrations.map(i =>
      i.id === id
        ? { ...i, status: i.status === 'connected' ? 'disconnected' : 'connected', lastSync: i.status === 'connected' ? 'Never' : 'Just now' }
        : i
    ));
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
          <button className="btn-primary text-xs flex items-center gap-1.5">
            <Plus size={12} /> Add Domain
          </button>
        </div>
        <div className="overflow-x-auto">
          <table className="data-table">
            <thead>
              <tr>
                <th>Domain</th>
                <th>Origin</th>
                <th>TLS Certificate</th>
                <th>Status</th>
                <th>Requests</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {domainList.map(domain => (
                <tr key={domain.id}>
                  <td>
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{domain.domain}</span>
                    </div>
                  </td>
                  <td className="font-mono text-xs">{domain.origin}</td>
                  <td>
                    <div className="flex items-center gap-1.5">
                      {domain.certStatus === 'valid' ? (
                        <CheckCircle2 size={14} className="text-green-400" />
                      ) : (
                        <AlertCircle size={14} className="text-yellow-400" />
                      )}
                      <span className="text-xs">
                        {domain.certStatus === 'valid' ? 'Valid' : 'Expiring'}
                      </span>
                      <span className="text-[10px]" style={{ color: 'var(--text-muted)' }}>({domain.certExpiry})</span>
                    </div>
                  </td>
                  <td>
                    <span className="badge badge-green">{domain.status}</span>
                  </td>
                  <td className="font-mono text-xs">{domain.requests}</td>
                  <td>
                    <button className="transition-colors" style={{ color: 'var(--text-muted)' }}>
                      <ExternalLink size={14} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* SIEM Integrations */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Webhook size={16} style={{ color: 'var(--text-muted)' }} />
          <div>
            <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>SIEM Integrations</h2>
            <p className="text-xs" style={{ color: 'var(--text-muted)' }}>Export logs and events to your security stack</p>
          </div>
        </div>

        <div className="grid md:grid-cols-2 gap-3">
          {integrations.map(integration => (
            <div key={integration.id} className="card p-4">
              <div className="flex items-start justify-between mb-3">
                <div className="flex items-center gap-3">
                  <div className="w-9 h-9 rounded-lg flex items-center justify-center text-sm font-bold" style={{
                    background: integration.name === 'Datadog' ? '#1a1000' :
                               integration.name === 'Splunk' ? '#1a0f00' :
                               integration.name === 'AWS S3' ? '#0a1a0a' : '#0a0a1a',
                    border: '1px solid var(--border-secondary)',
                    color: integration.name === 'Datadog' ? '#632ca6' :
                           integration.name === 'Splunk' ? '#e85d04' :
                           integration.name === 'AWS S3' ? '#ff9900' : '#0077cc',
                  }}>
                    {integration.name.charAt(0)}
                  </div>
                  <div>
                    <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{integration.name}</p>
                    <p className="text-[11px]" style={{ color: 'var(--text-muted)' }}>{integration.description}</p>
                  </div>
                </div>
                <div className="flex items-center gap-1.5">
                  {integration.status === 'connected' ? (
                    <CheckCircle2 size={14} className="text-green-400" />
                  ) : (
                    <XCircle size={14} style={{ color: 'var(--text-faint)' }} />
                  )}
                  <span className={`text-xs ${integration.status === 'connected' ? 'text-green-400' : ''}`} style={{ color: integration.status === 'connected' ? undefined : 'var(--text-muted)' }}>
                    {integration.status === 'connected' ? 'Connected' : 'Disconnected'}
                  </span>
                </div>
              </div>

              <div className="flex items-center justify-between pt-3 border-t" style={{ borderColor: 'var(--border-primary)' }}>
                <div className="flex gap-4">
                  <div>
                    <p className="text-[10px] uppercase" style={{ color: 'var(--text-muted)' }}>Last Sync</p>
                    <p className="text-xs font-mono" style={{ color: 'var(--text-primary)' }}>{integration.lastSync}</p>
                  </div>
                  <div>
                    <p className="text-[10px] uppercase" style={{ color: 'var(--text-muted)' }}>Events</p>
                    <p className="text-xs font-mono" style={{ color: 'var(--text-primary)' }}>{integration.events}</p>
                  </div>
                </div>
                <button
                  onClick={() => toggleIntegration(integration.id)}
                  className={`text-xs px-3 py-1.5 rounded font-medium ${
                    integration.status === 'connected'
                      ? 'text-blue-400 hover:text-blue-300'
                      : 'text-blue-400 hover:text-blue-300'
                  }`}
                  style={{ border: `1px solid ${integration.status === 'connected' ? 'var(--border-secondary)' : '#1e3a5f'}` }}
                >
                  {integration.status === 'connected' ? 'Configure' : 'Connect'}
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
