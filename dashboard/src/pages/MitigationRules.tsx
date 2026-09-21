import { useState } from 'react';
import { Plus, Trash2, ToggleLeft, ToggleRight, AlertCircle } from 'lucide-react';
import { managedRules as initialRules, exceptions as initialExceptions } from '../data/mockData';

const ruleFields = ['JA4 Fingerprint', 'Threat Score', 'IP Address', 'ASN', 'User-Agent', 'Request Path', 'Request Method', 'Geo', 'TLS Version'];
const operators = ['EQUALS', 'CONTAINS', 'MATCHES', '>', '<', '>=', '<='];
const actions = ['BLOCK', 'CHALLENGE', 'DECEIVE', 'PASS', 'LOG'];

interface RuleCondition {
  field: string;
  operator: string;
  value: string;
}

export default function MitigationRules() {
  const [rules, setRules] = useState(initialRules);
  const [exceptions, setExceptions] = useState(initialExceptions);
  const [showBuilder, setShowBuilder] = useState(false);
  const [ruleName, setRuleName] = useState('');
  const [conditions, setConditions] = useState<RuleCondition[]>([{ field: 'JA4 Fingerprint', operator: 'EQUALS', value: '' }]);
  const [action, setAction] = useState('BLOCK');

  const toggleRule = (id: string) => {
    setRules(rules.map(r => r.id === id ? { ...r, enabled: !r.enabled } : r));
  };

  const addCondition = () => {
    setConditions([...conditions, { field: 'JA4 Fingerprint', operator: 'EQUALS', value: '' }]);
  };

  const removeCondition = (idx: number) => {
    setConditions(conditions.filter((_, i) => i !== idx));
  };

  const updateCondition = (idx: number, key: keyof RuleCondition, value: string) => {
    const updated = [...conditions];
    updated[idx] = { ...updated[idx], [key]: value };
    setConditions(updated);
  };

  const removeException = (id: string) => {
    setExceptions(exceptions.filter(e => e.id !== id));
  };

  return (
    <div className="space-y-6 animate-in">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>Mitigation Rules</h1>
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>Define how threats are handled when detected</p>
        </div>
        <button onClick={() => setShowBuilder(!showBuilder)} className="btn-primary text-xs flex items-center gap-1.5">
          <Plus size={12} /> Custom Rule
        </button>
      </div>

      {/* Visual Rule Builder */}
      {showBuilder && (
        <div className="card p-5 animate-in">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Create Custom Rule</h3>
            <button onClick={() => setShowBuilder(false)} className="text-sm" style={{ color: 'var(--text-muted)' }}>×</button>
          </div>

          <div className="space-y-4">
            <div>
              <label className="text-xs block mb-1.5" style={{ color: 'var(--text-muted)' }}>Rule Name</label>
              <input
                type="text"
                value={ruleName}
                onChange={(e) => setRuleName(e.target.value)}
                placeholder="e.g., Block suspicious JA4 fingerprints"
                className="w-full px-3 py-2 text-sm rounded-md focus:outline-none focus:ring-1"
                style={{ background: 'var(--input-bg)', border: '1px solid var(--border-secondary)', color: 'var(--text-primary)' }}
              />
            </div>

            {/* Conditions */}
            <div>
              <label className="text-xs block mb-1.5" style={{ color: 'var(--text-muted)' }}>Conditions</label>
              <div className="space-y-2">
                {conditions.map((cond, idx) => (
                  <div key={idx} className="flex items-center gap-2">
                    {idx > 0 && <span className="text-xs w-8" style={{ color: 'var(--text-muted)' }}>AND</span>}
                    {idx === 0 && <span className="text-xs w-8" style={{ color: 'var(--text-muted)' }}>IF</span>}
                    <select
                      value={cond.field}
                      onChange={(e) => updateCondition(idx, 'field', e.target.value)}
                      className="px-2 py-1.5 text-xs rounded focus:outline-none"
                      style={{ background: 'var(--input-bg)', border: '1px solid var(--border-secondary)', color: 'var(--text-primary)' }}
                    >
                      {ruleFields.map(f => <option key={f} value={f}>{f}</option>)}
                    </select>
                    <select
                      value={cond.operator}
                      onChange={(e) => updateCondition(idx, 'operator', e.target.value)}
                      className="px-2 py-1.5 text-xs rounded focus:outline-none"
                      style={{ background: 'var(--input-bg)', border: '1px solid var(--border-secondary)', color: 'var(--text-primary)' }}
                    >
                      {operators.map(o => <option key={o} value={o}>{o}</option>)}
                    </select>
                    <input
                      type="text"
                      value={cond.value}
                      onChange={(e) => updateCondition(idx, 'value', e.target.value)}
                      placeholder="Value..."
                      className="flex-1 px-2 py-1.5 text-xs rounded font-mono focus:outline-none"
                      style={{ background: 'var(--input-bg)', border: '1px solid var(--border-secondary)', color: 'var(--text-primary)' }}
                    />
                    {conditions.length > 1 && (
                      <button onClick={() => removeCondition(idx)} className="hover:text-red-400" style={{ color: 'var(--text-muted)' }}>
                        <Trash2 size={14} />
                      </button>
                    )}
                  </div>
                ))}
              </div>
              <button onClick={addCondition} className="mt-2 text-xs text-blue-400 hover:text-blue-300 flex items-center gap-1">
                <Plus size={12} /> Add condition
              </button>
            </div>

            {/* Action */}
            <div>
              <label className="text-xs block mb-1.5" style={{ color: 'var(--text-muted)' }}>Then Action</label>
              <div className="flex gap-2">
                {actions.map(a => (
                  <button
                    key={a}
                    onClick={() => setAction(a)}
                    className={`px-3 py-1.5 text-xs rounded font-medium transition-colors ${
                      action === a ? '' : ''
                    }`}
                    style={{
                      background: action === a ? (a === 'BLOCK' ? '#7f1d1d' : a === 'CHALLENGE' ? '#713f12' : a === 'DECEIVE' ? '#7c2d12' : a === 'PASS' ? '#14532d' : 'var(--bg-tertiary)') : 'var(--input-bg)',
                      border: `1px solid ${action === a ? 'var(--border-secondary)' : 'var(--border-secondary)'}`,
                      color: action === a ? 'var(--text-primary)' : 'var(--text-muted)',
                    }}
                  >
                    {a}
                  </button>
                ))}
              </div>
            </div>

            {/* Rule Preview */}
            <div className="rounded p-3 font-mono text-xs" style={{ background: 'var(--code-bg)', border: '1px solid var(--border-primary)' }}>
              <span style={{ color: 'var(--text-muted)' }}>Rule Preview: </span>
              <span className="text-blue-400">IF</span>{' '}
              {conditions.map((c, i) => (
                <span key={i}>
                  {i > 0 && <><span style={{ color: 'var(--text-muted)' }}> AND </span></>}
                  <span className="text-green-400">[{c.field}]</span>{' '}
                  <span className="text-yellow-400">{c.operator}</span>{' '}
                  <span className="text-orange-400">[{c.value || '...'}]</span>
                </span>
              ))}{' '}
              <span className="text-blue-400">THEN</span>{' '}
              <span className="text-red-400">[{action}]</span>
            </div>

            <div className="flex justify-end gap-2 pt-2">
              <button onClick={() => setShowBuilder(false)} className="btn-secondary text-xs">Cancel</button>
              <button onClick={() => setShowBuilder(false)} className="btn-primary text-xs">Create Rule</button>
            </div>
          </div>
        </div>
      )}

      {/* Managed Rules */}
      <div className="card overflow-hidden">
        <div className="px-5 py-4 border-b flex items-center justify-between" style={{ borderColor: 'var(--border-primary)' }}>
          <div>
            <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Managed Rules</h2>
            <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>Pre-configured rules maintained by HakaiShield</p>
          </div>
          <span className="badge badge-blue">{rules.filter(r => r.enabled).length} active</span>
        </div>
        <div className="divide-y" style={{ borderColor: 'var(--border-primary)' }}>
          {rules.map(rule => (
            <div key={rule.id} className="px-5 py-3 flex items-center justify-between transition-colors" style={{ background: 'transparent' }}>
              <div className="flex items-center gap-3 flex-1 min-w-0">
                <button onClick={() => toggleRule(rule.id)} className="flex-shrink-0">
                  {rule.enabled ? (
                    <ToggleRight size={24} className="text-blue-400" />
                  ) : (
                    <ToggleLeft size={24} style={{ color: 'var(--text-faint)' }} />
                  )}
                </button>
                <div className="min-w-0">
                  <p className="text-sm font-medium" style={{ color: rule.enabled ? 'var(--text-primary)' : 'var(--text-muted)' }}>{rule.name}</p>
                  <p className="text-xs truncate" style={{ color: 'var(--text-muted)' }}>{rule.description}</p>
                </div>
              </div>
              <div className="flex items-center gap-4 flex-shrink-0 ml-4">
                <span className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>{rule.hits.toLocaleString()} hits</span>
                <span className={`badge ${rule.enabled ? 'badge-green' : 'badge-gray'}`}>
                  {rule.enabled ? 'Active' : 'Disabled'}
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Custom Exceptions */}
      <div className="card overflow-hidden">
        <div className="px-5 py-4 border-b flex items-center justify-between" style={{ borderColor: 'var(--border-primary)' }}>
          <div>
            <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Custom Exceptions</h2>
            <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>Whitelisted IPs, bots, and fingerprints</p>
          </div>
          <button className="btn-secondary text-xs flex items-center gap-1.5">
            <Plus size={12} /> Add Exception
          </button>
        </div>
        <div className="overflow-x-auto">
          <table className="data-table">
            <thead>
              <tr>
                <th>Type</th>
                <th>Value</th>
                <th>Reason</th>
                <th>Created</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {exceptions.map(ex => (
                <tr key={ex.id}>
                  <td><span className="badge badge-blue">{ex.type}</span></td>
                  <td className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>{ex.value}</td>
                  <td className="text-xs">{ex.reason}</td>
                  <td className="text-xs" style={{ color: 'var(--text-muted)' }}>{ex.createdAt}</td>
                  <td>
                    <button onClick={() => removeException(ex.id)} className="hover:text-red-400 transition-colors" style={{ color: 'var(--text-muted)' }}>
                      <Trash2 size={14} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
