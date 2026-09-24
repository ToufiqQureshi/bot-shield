import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { AlertCircle, ArrowRight } from 'lucide-react';
import { supabase } from '../lib/supabaseClient';

export default function Onboarding() {
  const navigate = useNavigate();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const continueToDashboard = async () => {
    setSubmitting(true);
    setError(null);
    try {
      const { error: updateError } = await supabase.auth.updateUser({
        data: { onboarding_complete: true },
      });
      if (updateError) throw updateError;
      navigate('/domains-siem');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not continue. Try again.');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center px-6 py-12" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="card p-8 w-full max-w-xl space-y-6">
        <a href="/landing" className="font-bold text-sm">hakaishield</a>
        <div>
          <h1 className="text-2xl font-semibold mb-2">Set up your pilot</h1>
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            Your account is ready. Domain setup is managed during the pilot so we can verify ownership,
            configure TLS and test your origin before routing visitors.
          </p>
        </div>
        <div className="flex items-start gap-3 rounded-lg p-4" style={{ background: 'var(--bg-tertiary)' }}>
          <AlertCircle size={18} className="text-yellow-400 shrink-0 mt-0.5" />
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
            Creating an account does not protect a site. Your domain will show as protected only after
            the operator completes DNS, certificate and traffic checks.
          </p>
        </div>
        <ol className="text-sm space-y-2" style={{ color: 'var(--text-secondary)' }}>
          <li>1. Share your domain and public origin URL with the pilot operator.</li>
          <li>2. We verify DNS ownership and prepare the TLS certificate.</li>
          <li>3. We test your site in shadow mode before enabling enforcement.</li>
        </ol>
        {error && <p role="alert" className="text-sm text-red-400">{error}</p>}
        <div className="flex flex-wrap gap-3">
          <button onClick={continueToDashboard} disabled={submitting} className="btn-primary px-5 py-2.5 text-sm flex items-center gap-2 disabled:opacity-60">
            {submitting ? 'Opening dashboard…' : 'Open dashboard'} <ArrowRight size={14} />
          </button>
          <a href="/contact" className="btn-secondary px-5 py-2.5 text-sm">Contact operator</a>
        </div>
      </div>
    </div>
  );
}
