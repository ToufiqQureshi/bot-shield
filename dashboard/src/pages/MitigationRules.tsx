import { useEffect, useState } from 'react';
import { Plus, Trash2, ToggleLeft, ToggleRight, AlertCircle, Lock } from 'lucide-react';
import {
  listRules,
  createRule,
  toggleRule as apiToggleRule,
  ApiError,
  type ManagedRule,
  type CustomRule,
  type RuleCondition,
} from '../lib/api';

const ruleFields = ['JA4 Fingerprint', 'Threat Score', 'IP Address', 'ASN', 'User-Agent', 'Request Path', 'Request Method', 'Geo', 'TLS Version'];
const operators = ['EQUALS', 'CONTAINS', 'MATCHES', '>', '<', '>=', '<='];
const actions = ['BLOCK', 'CHALLENGE', 'DECEIVE', 'PASS', 'LOG'];

export default function MitigationRules() {
  const [managedRules, setManagedRules] = useState<ManagedRule[]>([]);
  const [customRules, setCustomRules] = useState<CustomRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [showBuilder, setShowBuilder] = useState(false);
  const [ruleName, setRuleName] = useState('');
  const [conditions, setConditions] = useState<RuleCondition[]>([{ field: 'JA4 Fingerprint', operator: 'EQUALS', value: '' }]);
  const [action, setAction] = useState('BLOCK');
  const [createErr, setCreateErr] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const load = () => {
    setLoading(true);
    setLoadError(null);
    listRules()
      .then((r) => {
        setManagedRules(r.managedRules);
        setCustomRules(r.customRules);
      })
      .catch((err) => setLoadError(err instanceof ApiError ? err.message : 'Could not load rules.'))
      .finally(() => setLoading(false));
  };

  useEffect(load, []);

  const handleToggleCustom = async (rule: CustomRule) => {
    const next = !rule.enabled;
    setCustomRules(customRules.map(r => r.id === rule.id ? { ...r, enabled: next } : r));
    try {
      await apiToggleRule(rule.id, next);
    } catch {
      // Roll back on failure so the UI never claims a state the
      // backend didn't actually save.
      setCustomRules(customRules.map(r => r.id === rule.id ? { ...r, enabled: rule.enabled } : r));
    }
  };

  const addCondition = () => setConditions([...conditions, { field: 'JA4 Fingerprint', operator: 'EQUALS', value: '' }]);
  const removeCondition = (idx: number) => setConditions(conditions.filter((_, i) => i !== idx));
  const updateCondition = (idx: number, key: keyof RuleCondition, value: string) => {
    const updated = [...conditions];
    updated[idx] = { ...updated[idx], [key]: value };
    setConditions(updated);
  };

  const handleCreateRule = async () => {
    setCreateErr(null);
    if (!ruleName.trim() || conditions.some(c => !c.value.trim())) {
      setCreateErr('Give the rule a name and fill in every condition value.');
      return;
    }
    setCreating(true);
    try {
      const rule = await createRule(ruleName.trim(), conditions, action);
      setCustomRules([rule, ...customRules]);
      setShowBuilder(false);
      setRuleName('');
      setConditions([{ field: 'JA4 Fingerprint', operator: 'EQUALS', value: '' }]);
      setAction('BLOCK');
    } catch (err) {
      setCreateErr(err instanceof ApiError ? err.message : 'Could not create rule.');
    } finally {
      setCreating(false);
    }
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

      {loadError && (
        <div className="card p-4 flex items-center gap-2">
          <AlertCircle size={14} className="text-red-400" />
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>{loadError}</p>
        </div>
      )}

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
                    className="px-3 py-1.5 text-xs rounded font-medium transition-colors"
                    style={{
                      background: action === a ? (a === 'BLOCK' ? '#7f1d1d' : a === 'CHALLENGE' ? '#713f12' : a === 'DECEIVE' ? '#7c2d12' : a === 'PASS' ? '#14532d' : 'var(--bg-tertiary)') : 'var(--input-bg)',
                      border: '1px solid var(--border-secondary)',
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
                  {i > 0 && <span style={{ color: 'var(--text-muted)' }}> AND </span>}
                  <span className="text-green-400">[{c.field}]</span>{' '}
                  <span className="text-yellow-400">{c.operator}</span>{' '}
                  <span className="text-orange-400">[{c.value || '...'}]</span>
                </span>
              ))}{' '}
              <span className="text-blue-400">THEN</span>{' '}
              <span className="text-red-400">[{action}]</span>
            </div>

            {createErr && <p className="text-xs" style={{ color: 'var(--accent-red, #ef4444)' }}>{createErr}</p>}

            <div className="flex justify-end gap-2 pt-2">
              <button onClick={() => setShowBuilder(false)} className="btn-secondary text-xs">Cancel</button>
              <button onClick={handleCreateRule} disabled={creating} className="btn-primary text-xs disabled:opacity-60">
                {creating ? 'Creating…' : 'Create Rule'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Managed Rules */}
      <div className="card overflow-hidden">
        <div className="px-5 py-4 border-b flex items-center justify-between" style={{ borderColor: 'var(--border-primary)' }}>
          <div>
            <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Managed Rules</h2>
            <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>Always-on detection layers maintained by HakaiShield — not user-toggleable</p>
          </div>
          <span className="badge badge-blue">{managedRules.length} active</span>
        </div>
        <div className="divide-y" style={{ borderColor: 'var(--border-primary)' }}>
          {loading ? (
            <p className="px-5 py-6 text-xs" style={{ color: 'var(--text-muted)' }}>Loading…</p>
          ) : managedRules.map(rule => (
            <div key={rule.id} className="px-5 py-3 flex items-center justify-between">
              <div className="flex items-center gap-3 flex-1 min-w-0">
                <Lock size={16} className="flex-shrink-0" style={{ color: 'var(--text-faint)' }} />
                <div className="min-w-0">
                  <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{rule.name}</p>
                  <p className="text-xs truncate" style={{ color: 'var(--text-muted)' }}>{rule.description}</p>
                </div>
              </div>
              <span className="badge badge-green flex-shrink-0 ml-4">Active</span>
            </div>
          ))}
        </div>
      </div>

      {/* Custom Rules */}
      <div className="card overflow-hidden">
        <div className="px-5 py-4 border-b flex items-center justify-between" style={{ borderColor: 'var(--border-primary)' }}>
          <div>
            <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Custom Rules</h2>
            <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>Rules you've authored, evaluated in the order created</p>
          </div>
        </div>
        <div className="divide-y" style={{ borderColor: 'var(--border-primary)' }}>
          {!loading && customRules.length === 0 && (
            <p className="px-5 py-6 text-xs" style={{ color: 'var(--text-muted)' }}>No custom rules yet — use "Custom Rule" above to add one.</p>
          )}
          {customRules.map(rule => (
            <div key={rule.id} className="px-5 py-3 flex items-center justify-between">
              <div className="flex items-center gap-3 flex-1 min-w-0">
                <button onClick={() => handleToggleCustom(rule)} className="flex-shrink-0">
                  {rule.enabled ? (
                    <ToggleRight size={24} className="text-blue-400" />
                  ) : (
                    <ToggleLeft size={24} style={{ color: 'var(--text-faint)' }} />
                  )}
                </button>
                <div className="min-w-0">
                  <p className="text-sm font-medium" style={{ color: rule.enabled ? 'var(--text-primary)' : 'var(--text-muted)' }}>{rule.name}</p>
                  <p className="text-xs truncate font-mono" style={{ color: 'var(--text-muted)' }}>
                    {rule.conditions.map(c => `${c.field} ${c.operator} ${c.value}`).join(' AND ')} → {rule.action}
                  </p>
                </div>
              </div>
              <span className={`badge ${rule.enabled ? 'badge-green' : 'badge-gray'} flex-shrink-0 ml-4`}>
                {rule.enabled ? 'Active' : 'Disabled'}
              </span>
            </div>
          ))}
        </div>
      </div>

      {/* Custom Exceptions — not built yet */}
      <div className="card p-4 flex items-center gap-3">
        <AlertCircle size={16} className="text-yellow-400 shrink-0" />
        <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
          Custom exceptions (per-IP/JA4 allowlisting) aren't built yet — no endpoint exists to create or remove them.
        </p>
      </div>
    </div>
  );
}
