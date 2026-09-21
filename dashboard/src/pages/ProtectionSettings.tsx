import { useEffect, useState } from 'react';
import { Info, Shield, Zap, Ghost, Lock } from 'lucide-react';
import { getProtectionSettings, updateProtectionSettings, ApiError } from '../lib/api';

export default function ProtectionSettings() {
  const [blockThreshold, setBlockThreshold] = useState(90);
  const [challengeThreshold, setChallengeThreshold] = useState(50);
  const [challengeType, setChallengeType] = useState<'pow' | 'captcha'>('pow');
  const [honeypotEnabled, setHoneypotEnabled] = useState(true);
  const [tarpitDelay, setTarpitDelay] = useState(30);
  const [sqliProtection, setSqliProtection] = useState(true);
  const [xssProtection, setXssProtection] = useState(true);
  const [rateLimitEnabled, setRateLimitEnabled] = useState(true);
  const [rateLimitRpm, setRateLimitRpm] = useState(100);

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    getProtectionSettings()
      .then((s) => {
        setBlockThreshold(s.blockThreshold);
        setChallengeThreshold(s.challengeThreshold);
        setChallengeType(s.challengeType === 'captcha' ? 'captcha' : 'pow');
        setHoneypotEnabled(s.honeypotEnabled);
      })
      .catch((err) => setError(err instanceof ApiError ? err.message : 'Could not load settings.'))
      .finally(() => setLoading(false));
  }, []);

  const handleSave = async () => {
    setError(null);
    setSaved(false);
    setSaving(true);
    try {
      await updateProtectionSettings({ blockThreshold, challengeThreshold, challengeType, honeypotEnabled });
      setSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Could not save settings.');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6 animate-in">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>Protection Settings</h1>
          <p className="text-sm" style={{ color: 'var(--text-muted)' }}>Configure security modules and edge behavior</p>
        </div>
        <div className="flex items-center gap-3">
          {saved && <span className="text-xs text-green-400">Saved</span>}
          {error && <span className="text-xs" style={{ color: 'var(--accent-red, #ef4444)' }}>{error}</span>}
          <button onClick={handleSave} disabled={loading || saving} className="btn-primary text-xs px-4 py-2 disabled:opacity-60">
            {saving ? 'Saving…' : 'Save changes'}
          </button>
        </div>
      </div>

      <div className="grid lg:grid-cols-2 gap-6">
        {/* Threat Scoring Thresholds */}
        <div className="card p-5">
          <div className="flex items-center gap-2 mb-4">
            <Shield size={16} className="text-blue-400" />
            <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Threat Scoring Thresholds</h3>
          </div>
          <p className="text-xs mb-5" style={{ color: 'var(--text-muted)' }}>Set sensitivity levels for automated decisions</p>

          <div className="space-y-6">
            {/* Block Threshold */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <label className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>Block Threshold</label>
                <span className="font-mono text-sm text-red-400">{blockThreshold}</span>
              </div>
              <input
                type="range"
                min="50"
                max="100"
                value={blockThreshold}
                onChange={(e) => setBlockThreshold(Number(e.target.value))}
                className="w-full"
              />
              <div className="flex justify-between mt-1">
                <span className="text-[10px]" style={{ color: 'var(--text-muted)' }}>50</span>
                <span className="text-[10px]" style={{ color: 'var(--text-muted)' }}>100</span>
              </div>
              <p className="text-[11px] mt-1" style={{ color: 'var(--text-muted)' }}>Requests scoring above this value are immediately blocked</p>
            </div>

            {/* Challenge Threshold */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <label className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>Challenge Threshold</label>
                <span className="font-mono text-sm text-yellow-400">{challengeThreshold}</span>
              </div>
              <input
                type="range"
                min="10"
                max="80"
                value={challengeThreshold}
                onChange={(e) => setChallengeThreshold(Number(e.target.value))}
                className="w-full"
              />
              <div className="flex justify-between mt-1">
                <span className="text-[10px]" style={{ color: 'var(--text-muted)' }}>10</span>
                <span className="text-[10px]" style={{ color: 'var(--text-muted)' }}>80</span>
              </div>
              <p className="text-[11px] mt-1" style={{ color: 'var(--text-muted)' }}>Requests scoring above this value receive a challenge</p>
            </div>

            {/* Visual scale */}
            <div className="rounded-lg p-3" style={{ background: 'var(--code-bg)', border: '1px solid var(--border-primary)' }}>
              <div className="h-3 rounded-full overflow-hidden flex">
                <div className="bg-green-500/30" style={{ width: `${challengeThreshold}%` }}></div>
                <div className="bg-yellow-500/30" style={{ width: `${blockThreshold - challengeThreshold}%` }}></div>
                <div className="bg-red-500/30 flex-1"></div>
              </div>
              <div className="flex justify-between mt-1.5 text-[10px]">
                <span className="text-green-400">PASS (0-{challengeThreshold})</span>
                <span className="text-yellow-400">CHALLENGE ({challengeThreshold}-{blockThreshold})</span>
                <span className="text-red-400">BLOCK ({blockThreshold}+)</span>
              </div>
            </div>
          </div>
        </div>

        {/* Challenge Configuration */}
        <div className="card p-5">
          <div className="flex items-center gap-2 mb-4">
            <Zap size={16} className="text-yellow-400" />
            <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Challenge Configuration</h3>
          </div>
          <p className="text-xs mb-5" style={{ color: 'var(--text-muted)' }}>Choose how suspicious clients are challenged</p>

          <div className="space-y-3">
            <button
              onClick={() => setChallengeType('pow')}
              className="w-full text-left rounded-lg p-4 transition-colors"
              style={{
                background: challengeType === 'pow' ? 'var(--bg-tertiary)' : 'transparent',
                border: `1px solid ${challengeType === 'pow' ? 'var(--border-secondary)' : 'var(--border-secondary)'}`,
              }}
            >
              <div className="flex items-center justify-between mb-1">
                <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>Invisible Proof-of-Work</span>
                {challengeType === 'pow' && <span className="badge badge-blue">Selected</span>}
              </div>
              <p className="text-xs" style={{ color: 'var(--text-muted)' }}>Client solves a computational challenge silently. No user interaction required. Adds ~200ms latency to bots.</p>
            </button>

            <button
              onClick={() => setChallengeType('captcha')}
              className="w-full text-left rounded-lg p-4 transition-colors"
              style={{
                background: challengeType === 'captcha' ? 'var(--bg-tertiary)' : 'transparent',
                border: `1px solid ${challengeType === 'captcha' ? 'var(--border-secondary)' : 'var(--border-secondary)'}`,
              }}
            >
              <div className="flex items-center justify-between mb-1">
                <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>Interactive Turnstile CAPTCHA</span>
                {challengeType === 'captcha' && <span className="badge badge-blue">Selected</span>}
              </div>
              <p className="text-xs" style={{ color: 'var(--text-muted)' }}>Present a visual challenge to verify human interaction. Best for high-value endpoints.</p>
            </button>
          </div>
        </div>

        {/* Deception Engine */}
        <div className="card p-5">
          <div className="flex items-center gap-2 mb-4">
            <Ghost size={16} className="text-orange-400" />
            <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Deception Engine</h3>
            <span className="badge badge-orange">Beta</span>
          </div>
          <p className="text-xs mb-5" style={{ color: 'var(--text-muted)' }}>Route malicious bots to honeypots and tarpits</p>

          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm" style={{ color: 'var(--text-primary)' }}>Enable Honeypot Routing</p>
                <p className="text-xs" style={{ color: 'var(--text-muted)' }}>Serve fake data to waste bot resources</p>
              </div>
              <div
                className={`toggle ${honeypotEnabled ? 'active' : ''}`}
                onClick={() => setHoneypotEnabled(!honeypotEnabled)}
              />
            </div>

            {honeypotEnabled && (
              <div className="pl-4 border-l-2 border-orange-500/30 space-y-3">
                <div>
                  <div className="flex items-center justify-between mb-1">
                    <label className="text-xs" style={{ color: 'var(--text-primary)' }}>Tarpit Delay</label>
                    <span className="font-mono text-xs text-orange-400">{tarpitDelay}s</span>
                  </div>
                  <input
                    type="range"
                    min="5"
                    max="60"
                    value={tarpitDelay}
                    onChange={(e) => setTarpitDelay(Number(e.target.value))}
                    className="w-full"
                  />
                  <p className="text-[11px] mt-1" style={{ color: 'var(--text-muted)' }}>Delay responses to waste bot compute time</p>
                </div>

                <div className="rounded p-3 flex items-start gap-2" style={{ background: 'var(--code-bg)', border: '1px solid var(--border-primary)' }}>
                  <Info size={14} className="flex-shrink-0 mt-0.5" style={{ color: 'var(--text-muted)' }} />
                  <p className="text-[11px]" style={{ color: 'var(--text-muted)' }}>
                    The trap link is injected automatically into deceived responses — there's no configurable
                    endpoint list yet (tarpit delay and custom honeypot paths aren't wired to the backend).
                  </p>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* WAF Settings */}
        <div className="card p-5">
          <div className="flex items-center gap-2 mb-4">
            <Lock size={16} style={{ color: 'var(--text-muted)' }} />
            <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>WAF Settings</h3>
            <span className="badge badge-gray">Coming Soon</span>
          </div>
          <p className="text-xs mb-5" style={{ color: 'var(--text-muted)' }}>Web Application Firewall protections</p>

          <div className="space-y-4">
            <div className="flex items-center justify-between opacity-50">
              <div>
                <p className="text-sm" style={{ color: 'var(--text-primary)' }}>SQL Injection Protection</p>
                <p className="text-xs" style={{ color: 'var(--text-muted)' }}>Detect and block SQLi payloads</p>
              </div>
              <div className={`toggle ${sqliProtection ? 'active' : ''}`} onClick={() => setSqliProtection(!sqliProtection)} />
            </div>

            <div className="flex items-center justify-between opacity-50">
              <div>
                <p className="text-sm" style={{ color: 'var(--text-primary)' }}>XSS Protection</p>
                <p className="text-xs" style={{ color: 'var(--text-muted)' }}>Block cross-site scripting attempts</p>
              </div>
              <div className={`toggle ${xssProtection ? 'active' : ''}`} onClick={() => setXssProtection(!xssProtection)} />
            </div>

            <div className="opacity-50">
              <div className="flex items-center justify-between mb-2">
                <div>
                  <p className="text-sm" style={{ color: 'var(--text-primary)' }}>Rate Limiting</p>
                  <p className="text-xs" style={{ color: 'var(--text-muted)' }}>Max requests per minute per IP</p>
                </div>
                <div className={`toggle ${rateLimitEnabled ? 'active' : ''}`} onClick={() => setRateLimitEnabled(!rateLimitEnabled)} />
              </div>
              {rateLimitEnabled && (
                <div className="flex items-center gap-2">
                  <input
                    type="number"
                    value={rateLimitRpm}
                    onChange={(e) => setRateLimitRpm(Number(e.target.value))}
                    className="w-20 px-2 py-1 text-xs font-mono rounded focus:outline-none"
                    style={{ background: 'var(--input-bg)', border: '1px solid var(--border-secondary)', color: 'var(--text-primary)' }}
                  />
                  <span className="text-xs" style={{ color: 'var(--text-muted)' }}>requests/min</span>
                </div>
              )}
            </div>

            <div className="rounded p-3 flex items-start gap-2" style={{ background: 'var(--code-bg)', border: '1px solid var(--border-primary)' }}>
              <Info size={14} className="flex-shrink-0 mt-0.5" style={{ color: 'var(--text-muted)' }} />
              <p className="text-[11px]" style={{ color: 'var(--text-muted)' }}>WAF rules are currently in development. Bot detection and threat scoring remain fully active.</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
