import { useEffect, useState } from 'react';
import { AlertCircle, Lock } from 'lucide-react';
import { listRules, ApiError, type ManagedRule, type CustomRule } from '../lib/api';

export default function MitigationRules() {
  const [managedRules, setManagedRules] = useState<ManagedRule[]>([]);
  const [customRules, setCustomRules] = useState<CustomRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    listRules()
      .then((rules) => {
        if (cancelled) return;
        setManagedRules(rules.managedRules);
        setCustomRules(rules.customRules);
      })
      .catch((err) => !cancelled && setError(err instanceof ApiError ? err.message : 'Could not load rules.'))
      .finally(() => !cancelled && setLoading(false));
    return () => { cancelled = true; };
  }, []);

  return (
    <div className="space-y-6 animate-in">
      <div>
        <h1 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>Mitigation rules</h1>
        <p className="text-sm" style={{ color: 'var(--text-muted)' }}>Review the pilot policy with your operator.</p>
      </div>
      <div className="card p-5 flex items-start gap-3">
        <AlertCircle size={18} className="text-yellow-400 shrink-0 mt-0.5" />
        <div className="space-y-2">
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            Saved custom rules are drafts for shadow review; their enabled flag does not mean they are
            enforcing traffic. The operator verifies a policy against real evidence before activation.
          </p>
          <a href="/contact" className="text-xs underline">Request a rule change</a>
        </div>
      </div>
      {error && <p role="alert" className="text-sm text-red-400">{error}</p>}
      <div className="card overflow-hidden">
        <div className="px-5 py-4 border-b" style={{ borderColor: 'var(--border-primary)' }}>
          <h2 className="text-sm font-semibold">Managed detection</h2>
        </div>
        {loading ? (
          <p className="p-5 text-xs" style={{ color: 'var(--text-muted)' }}>Loading…</p>
        ) : managedRules.length === 0 ? (
          <p className="p-5 text-xs" style={{ color: 'var(--text-muted)' }}>No managed rule listing is available.</p>
        ) : managedRules.map((rule) => (
          <div key={rule.id} className="px-5 py-3 border-b flex items-start gap-3" style={{ borderColor: 'var(--border-primary)' }}>
            <Lock size={16} className="shrink-0 mt-0.5" style={{ color: 'var(--text-muted)' }} />
            <div>
              <p className="text-sm font-medium">{rule.name}</p>
              <p className="text-xs" style={{ color: 'var(--text-muted)' }}>{rule.description}</p>
            </div>
          </div>
        ))}
      </div>
      {customRules.length > 0 && (
        <div className="card overflow-hidden">
          <div className="px-5 py-4 border-b" style={{ borderColor: 'var(--border-primary)' }}>
            <h2 className="text-sm font-semibold">Saved rule drafts</h2>
          </div>
          {customRules.map((rule) => (
            <div key={rule.id} className="px-5 py-3 border-b" style={{ borderColor: 'var(--border-primary)' }}>
              <p className="text-sm font-medium">{rule.name}</p>
              <p className="text-xs font-mono" style={{ color: 'var(--text-muted)' }}>
                {rule.conditions.map((condition) => `${condition.field} ${condition.operator} ${condition.value}`).join(' AND ')} → {rule.action}
              </p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
