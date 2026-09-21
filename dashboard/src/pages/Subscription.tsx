import { useTheme } from '../context/ThemeContext';
import { ArrowRight, Check } from 'lucide-react';

export default function Subscription() {
  const { theme, toggleTheme } = useTheme();

  const plans = [
    {
      name: 'Starter',
      price: '$200',
      period: '/mo',
      description: 'For growing sites that need bot protection now.',
      features: [
        '1M requests/month',
        '3 protected domains',
        'JA4 TLS fingerprinting',
        'Multi-layer scoring',
        'Shadow mode',
        'Evidence trail (24h)',
        'JS challenge',
        'Email support',
      ],
      current: false,
    },
    {
      name: 'Growth',
      price: '$500',
      period: '/mo',
      description: 'For businesses where bots are a revenue leak.',
      features: [
        '10M requests/month',
        '10 protected domains',
        'Everything in Starter',
        'Behavioral scoring',
        'Deception engine',
        'Evidence trail (30 days)',
        'Custom rules',
        'SIEM integrations',
        'Rate limiting per endpoint',
        'Priority support',
      ],
      current: true,
    },
    {
      name: 'Enterprise',
      price: 'Custom',
      period: '',
      description: 'Self-hosted. For regulated industries.',
      features: [
        'Unlimited requests',
        'Unlimited domains',
        'Everything in Growth',
        'Self-hosted deployment',
        'Data residency',
        'SSO / SAML',
        'Dedicated account manager',
        'SLA guarantee',
        'Custom integrations',
        'Fingerprint database access',
      ],
      current: false,
    },
  ];

  return (
    <div className="min-h-screen" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      {/* Nav */}
      <nav className="sticky top-0 z-50 border-b" style={{ borderColor: 'var(--border-primary)', background: theme === 'dark' ? 'rgba(0,0,0,0.8)' : 'rgba(255,255,255,0.8)', backdropFilter: 'blur(12px)' }}>
        <div className="max-w-5xl mx-auto px-6 h-14 flex items-center justify-between">
          <a href="/" className="font-bold text-sm tracking-tight">hakaishield</a>
          <div className="flex items-center gap-4">
            <button onClick={toggleTheme} className="text-sm" style={{ color: 'var(--text-muted)' }}>
              {theme === 'dark' ? 'Light' : 'Dark'}
            </button>
            <a href="/" className="text-sm" style={{ color: 'var(--text-secondary)' }}>Dashboard</a>
          </div>
        </div>
      </nav>

      <div className="max-w-5xl mx-auto px-6 py-16">
        <div className="mb-12">
          <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>
            // subscription
          </p>
          <h1 className="text-4xl font-bold mb-4 tracking-tight">Manage your plan</h1>
          <p className="text-lg" style={{ color: 'var(--text-secondary)' }}>
            You're currently on the <strong className="text-white">Growth</strong> plan.
          </p>
        </div>

        {/* Current Plan Summary */}
        <div className="card p-6 mb-8">
          <div className="flex items-center justify-between mb-4">
            <div>
              <p className="text-xs font-mono mb-1" style={{ color: 'var(--text-muted)' }}>CURRENT PLAN</p>
              <p className="text-2xl font-bold">Growth</p>
            </div>
            <div className="text-right">
              <p className="text-2xl font-bold">$500<span className="text-sm font-normal" style={{ color: 'var(--text-muted)' }}>/mo</span></p>
              <p className="text-xs" style={{ color: 'var(--text-muted)' }}>Next billing: Oct 1, 2026</p>
            </div>
          </div>
          <div className="grid grid-cols-3 gap-4 pt-4 border-t" style={{ borderColor: 'var(--border-primary)' }}>
            <div>
              <p className="text-xs mb-1" style={{ color: 'var(--text-muted)' }}>Requests used</p>
              <p className="text-lg font-bold font-mono">4.2M <span className="text-xs font-normal" style={{ color: 'var(--text-muted)' }}>/ 10M</span></p>
            </div>
            <div>
              <p className="text-xs mb-1" style={{ color: 'var(--text-muted)' }}>Domains</p>
              <p className="text-lg font-bold font-mono">7 <span className="text-xs font-normal" style={{ color: 'var(--text-muted)' }}>/ 10</span></p>
            </div>
            <div>
              <p className="text-xs mb-1" style={{ color: 'var(--text-muted)' }}>Billing cycle</p>
              <p className="text-lg font-bold">Monthly</p>
            </div>
          </div>
        </div>

        {/* Plans */}
        <h2 className="text-2xl font-bold mb-6 tracking-tight">Change plan</h2>
        <div className="grid md:grid-cols-3 gap-px mb-8" style={{ background: 'var(--border-primary)' }}>
          {plans.map((plan, i) => (
            <div key={i} className="p-6 relative" style={{ background: plan.current ? 'var(--bg-tertiary)' : 'var(--bg-primary)' }}>
              {plan.current && (
                <div className="absolute top-4 right-4">
                  <span className="text-[10px] font-mono px-2 py-0.5 rounded" style={{ background: 'var(--accent-blue)', color: 'white' }}>CURRENT</span>
                </div>
              )}
              <p className="text-xs font-mono mb-4" style={{ color: 'var(--text-muted)' }}>{plan.name.toUpperCase()}</p>
              <div className="mb-6">
                <span className="text-4xl font-bold">{plan.price}</span>
                <span className="text-sm" style={{ color: 'var(--text-muted)' }}>{plan.period}</span>
              </div>
              <p className="text-sm mb-6" style={{ color: 'var(--text-secondary)' }}>
                {plan.description}
              </p>
              {!plan.current && (
                <a href={`/payment?plan=${plan.name.toLowerCase()}`} className="btn-secondary w-full py-2.5 text-sm text-center block mb-6">
                  {plan.name === 'Enterprise' ? 'Contact sales' : `Switch to ${plan.name}`}
                </a>
              )}
              {plan.current && (
                <div className="w-full py-2.5 text-sm text-center mb-6 rounded" style={{ background: 'var(--bg-elevated)', color: 'var(--text-muted)' }}>
                  Current plan
                </div>
              )}
              <ul className="space-y-3 text-sm" style={{ color: 'var(--text-secondary)' }}>
                {plan.features.map((feature, j) => (
                  <li key={j} className="flex items-start gap-2">
                    <Check size={14} className="text-green-400 flex-shrink-0 mt-0.5" />
                    <span>{feature}</span>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        {/* Billing History */}
        <div className="card overflow-hidden">
          <div className="px-6 py-4 border-b flex items-center justify-between" style={{ borderColor: 'var(--border-primary)' }}>
            <h3 className="text-sm font-semibold">Billing history</h3>
            <button className="btn-secondary text-xs">Download all</button>
          </div>
          <table className="w-full">
            <thead>
              <tr style={{ background: 'var(--bg-tertiary)' }}>
                <th className="text-left px-6 py-3 text-xs font-medium" style={{ color: 'var(--text-muted)' }}>Date</th>
                <th className="text-left px-6 py-3 text-xs font-medium" style={{ color: 'var(--text-muted)' }}>Description</th>
                <th className="text-left px-6 py-3 text-xs font-medium" style={{ color: 'var(--text-muted)' }}>Amount</th>
                <th className="text-left px-6 py-3 text-xs font-medium" style={{ color: 'var(--text-muted)' }}>Status</th>
                <th className="text-left px-6 py-3 text-xs font-medium" style={{ color: 'var(--text-muted)' }}></th>
              </tr>
            </thead>
            <tbody>
              {[
                { date: 'Sep 1, 2026', desc: 'Growth plan', amount: '$500.00', status: 'Paid' },
                { date: 'Aug 1, 2026', desc: 'Growth plan', amount: '$500.00', status: 'Paid' },
                { date: 'Jul 1, 2026', desc: 'Growth plan', amount: '$500.00', status: 'Paid' },
              ].map((invoice, i) => (
                <tr key={i} className="border-t" style={{ borderColor: 'var(--border-primary)' }}>
                  <td className="px-6 py-3 text-sm">{invoice.date}</td>
                  <td className="px-6 py-3 text-sm">{invoice.desc}</td>
                  <td className="px-6 py-3 text-sm font-mono">{invoice.amount}</td>
                  <td className="px-6 py-3">
                    <span className="badge badge-green text-[10px]">{invoice.status}</span>
                  </td>
                  <td className="px-6 py-3 text-right">
                    <button className="text-xs" style={{ color: 'var(--accent-blue)' }}>Download</button>
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
