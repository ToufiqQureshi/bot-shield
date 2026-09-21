import { useState } from 'react';
import { useTheme } from '../context/ThemeContext';
import { ArrowRight, ArrowLeft } from 'lucide-react';

export default function Onboarding() {
  const { theme } = useTheme();
  const [step, setStep] = useState(1);
  const [formData, setFormData] = useState({
    useCase: '',
    monthlyVisitors: '',
    website: '',
    botProblem: '',
    teamSize: '',
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    // In production, save onboarding data
    console.log('Onboarding complete:', formData);
    // Redirect to dashboard
    window.location.href = '/';
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  const nextStep = () => setStep(step + 1);
  const prevStep = () => setStep(step - 1);

  return (
    <div className="min-h-screen flex items-center justify-center py-12" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="w-full max-w-2xl px-6">
        <div className="text-center mb-8">
          <a href="/landing" className="font-bold text-xl tracking-tight">hakaishield</a>
        </div>

        {/* Progress indicator */}
        <div className="flex items-center justify-center gap-2 mb-8">
          {[1, 2, 3].map((s) => (
            <div
              key={s}
              className="h-1 w-16 rounded-full transition-colors"
              style={{ background: s <= step ? 'var(--accent-blue)' : 'var(--border-primary)' }}
            />
          ))}
        </div>

        <div className="card p-8">
          <form onSubmit={handleSubmit}>
            {/* Step 1: Use Case */}
            {step === 1 && (
              <div>
                <h1 className="text-2xl font-bold mb-2 tracking-tight">What brings you to hakaishield?</h1>
                <p className="text-sm mb-8" style={{ color: 'var(--text-secondary)' }}>
                  This helps us personalize your experience
                </p>

                <div className="space-y-3">
                  {[
                    { value: 'ecommerce', label: 'E-commerce', desc: 'Protect pricing, inventory, checkout' },
                    { value: 'ticketing', label: 'Ticketing & Booking', desc: 'Prevent scalping and hoarding' },
                    { value: 'api', label: 'API Platform', desc: 'Stop API abuse and reduce costs' },
                    { value: 'content', label: 'Content & Media', desc: 'Protect articles, videos, listings' },
                    { value: 'saas', label: 'SaaS Application', desc: 'Prevent account takeover, scraping' },
                    { value: 'other', label: 'Other', desc: 'Something else' },
                  ].map((option) => (
                    <label
                      key={option.value}
                      className="flex items-start gap-3 p-4 rounded cursor-pointer transition-colors"
                      style={{
                        background: formData.useCase === option.value ? 'var(--bg-tertiary)' : 'transparent',
                        border: `1px solid ${formData.useCase === option.value ? 'var(--accent-blue)' : 'var(--border-primary)'}`,
                      }}
                    >
                      <input
                        type="radio"
                        name="useCase"
                        value={option.value}
                        checked={formData.useCase === option.value}
                        onChange={handleChange}
                        className="mt-1"
                      />
                      <div>
                        <p className="font-medium text-sm">{option.label}</p>
                        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>{option.desc}</p>
                      </div>
                    </label>
                  ))}
                </div>

                <div className="mt-8 flex justify-end">
                  <button type="button" onClick={nextStep} className="group btn-primary px-6 py-2.5 text-sm flex items-center gap-2">
                    Continue
                    <ArrowRight size={14} className="transition-transform group-hover:translate-x-0.5" />
                  </button>
                </div>
              </div>
            )}

            {/* Step 2: Traffic & Website */}
            {step === 2 && (
              <div>
                <h1 className="text-2xl font-bold mb-2 tracking-tight">Tell us about your traffic</h1>
                <p className="text-sm mb-8" style={{ color: 'var(--text-secondary)' }}>
                  This helps us recommend the right plan
                </p>

                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium mb-2">Monthly visitors</label>
                    <select
                      name="monthlyVisitors"
                      value={formData.monthlyVisitors}
                      onChange={handleChange}
                      required
                      className="w-full px-4 py-2.5 rounded text-sm"
                      style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                    >
                      <option value="">Select range</option>
                      <option value="<100k">&lt; 100K</option>
                      <option value="100k-500k">100K - 500K</option>
                      <option value="500k-2m">500K - 2M</option>
                      <option value="2m-10m">2M - 10M</option>
                      <option value=">10m">&gt; 10M</option>
                    </select>
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-2">Website URL</label>
                    <input
                      type="url"
                      name="website"
                      value={formData.website}
                      onChange={handleChange}
                      required
                      className="w-full px-4 py-2.5 rounded text-sm"
                      style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                      placeholder="https://yoursite.com"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-2">Team size</label>
                    <select
                      name="teamSize"
                      value={formData.teamSize}
                      onChange={handleChange}
                      required
                      className="w-full px-4 py-2.5 rounded text-sm"
                      style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                    >
                      <option value="">Select size</option>
                      <option value="solo">Just me</option>
                      <option value="2-10">2-10 people</option>
                      <option value="11-50">11-50 people</option>
                      <option value="51-200">51-200 people</option>
                      <option value="200+">200+ people</option>
                    </select>
                  </div>
                </div>

                <div className="mt-8 flex justify-between">
                  <button type="button" onClick={prevStep} className="btn-secondary px-6 py-2.5 text-sm flex items-center gap-2">
                    <ArrowLeft size={14} />
                    Back
                  </button>
                  <button type="button" onClick={nextStep} className="group btn-primary px-6 py-2.5 text-sm flex items-center gap-2">
                    Continue
                    <ArrowRight size={14} className="transition-transform group-hover:translate-x-0.5" />
                  </button>
                </div>
              </div>
            )}

            {/* Step 3: Bot Problem */}
            {step === 3 && (
              <div>
                <h1 className="text-2xl font-bold mb-2 tracking-tight">What's your biggest bot concern?</h1>
                <p className="text-sm mb-8" style={{ color: 'var(--text-secondary)' }}>
                  We'll prioritize features based on your needs
                </p>

                <div className="space-y-3">
                  {[
                    { value: 'pricing-scraping', label: 'Pricing scraping', desc: 'Competitors copying your prices' },
                    { value: 'inventory-hoarding', label: 'Inventory hoarding', desc: 'Bots buying up stock/tickets' },
                    { value: 'api-abuse', label: 'API abuse', desc: 'Excessive API calls driving up costs' },
                    { value: 'account-takeover', label: 'Account takeover', desc: 'Credential stuffing attacks' },
                    { value: 'content-scraping', label: 'Content scraping', desc: 'Copying articles, listings, data' },
                    { value: 'not-sure', label: 'Not sure yet', desc: 'Just exploring bot protection' },
                  ].map((option) => (
                    <label
                      key={option.value}
                      className="flex items-start gap-3 p-4 rounded cursor-pointer transition-colors"
                      style={{
                        background: formData.botProblem === option.value ? 'var(--bg-tertiary)' : 'transparent',
                        border: `1px solid ${formData.botProblem === option.value ? 'var(--accent-blue)' : 'var(--border-primary)'}`,
                      }}
                    >
                      <input
                        type="radio"
                        name="botProblem"
                        value={option.value}
                        checked={formData.botProblem === option.value}
                        onChange={handleChange}
                        className="mt-1"
                      />
                      <div>
                        <p className="font-medium text-sm">{option.label}</p>
                        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>{option.desc}</p>
                      </div>
                    </label>
                  ))}
                </div>

                <div className="mt-8 flex justify-between">
                  <button type="button" onClick={prevStep} className="btn-secondary px-6 py-2.5 text-sm flex items-center gap-2">
                    <ArrowLeft size={14} />
                    Back
                  </button>
                  <button type="submit" className="group btn-primary px-6 py-2.5 text-sm flex items-center gap-2">
                    Complete setup
                    <ArrowRight size={14} className="transition-transform group-hover:translate-x-0.5" />
                  </button>
                </div>
              </div>
            )}
          </form>
        </div>

        <div className="mt-6 text-center">
          <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
            Step {step} of 3
          </p>
        </div>
      </div>
    </div>
  );
}
