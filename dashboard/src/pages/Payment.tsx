import { useState } from 'react';
import { useTheme } from '../context/ThemeContext';
import { ArrowRight, Lock } from 'lucide-react';

export default function Payment() {
  const { theme, toggleTheme } = useTheme();
  const [processing, setProcessing] = useState(false);
  const [formData, setFormData] = useState({
    cardNumber: '',
    expiry: '',
    cvc: '',
    name: '',
    country: 'US',
    zip: '',
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setProcessing(true);
    // In production, this would process payment
    setTimeout(() => {
      console.log('Payment processed:', formData);
      window.location.href = '/';
    }, 2000);
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  return (
    <div className="min-h-screen" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      {/* Nav */}
      <nav className="sticky top-0 z-50 border-b" style={{ borderColor: 'var(--border-primary)', background: theme === 'dark' ? 'rgba(0,0,0,0.8)' : 'rgba(255,255,255,0.8)', backdropFilter: 'blur(12px)' }}>
        <div className="max-w-5xl mx-auto px-6 h-14 flex items-center justify-between">
          <a href="/landing" className="font-bold text-sm tracking-tight">hakaishield</a>
          <div className="flex items-center gap-4">
            <button onClick={toggleTheme} className="text-sm" style={{ color: 'var(--text-muted)' }}>
              {theme === 'dark' ? 'Light' : 'Dark'}
            </button>
            <a href="/pricing" className="text-sm" style={{ color: 'var(--text-secondary)' }}>← Back to pricing</a>
          </div>
        </div>
      </nav>

      <div className="max-w-4xl mx-auto px-6 py-16">
        <div className="grid md:grid-cols-5 gap-8">
          {/* Payment Form */}
          <div className="md:col-span-3">
            <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>
              // payment
            </p>
            <h1 className="text-3xl font-bold mb-2 tracking-tight">Complete your purchase</h1>
            <p className="text-sm mb-8" style={{ color: 'var(--text-secondary)' }}>
              You're subscribing to the <strong className="text-white">Growth</strong> plan
            </p>

            <form onSubmit={handleSubmit} className="space-y-6">
              <div className="card p-6">
                <h2 className="text-sm font-semibold mb-4">Payment method</h2>
                
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium mb-2">Card number</label>
                    <input
                      type="text"
                      name="cardNumber"
                      value={formData.cardNumber}
                      onChange={handleChange}
                      required
                      placeholder="1234 5678 9012 3456"
                      className="w-full px-4 py-2.5 rounded text-sm font-mono"
                      style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                    />
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium mb-2">Expiry</label>
                      <input
                        type="text"
                        name="expiry"
                        value={formData.expiry}
                        onChange={handleChange}
                        required
                        placeholder="MM / YY"
                        className="w-full px-4 py-2.5 rounded text-sm font-mono"
                        style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium mb-2">CVC</label>
                      <input
                        type="text"
                        name="cvc"
                        value={formData.cvc}
                        onChange={handleChange}
                        required
                        placeholder="123"
                        className="w-full px-4 py-2.5 rounded text-sm font-mono"
                        style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                      />
                    </div>
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-2">Name on card</label>
                    <input
                      type="text"
                      name="name"
                      value={formData.name}
                      onChange={handleChange}
                      required
                      placeholder="John Doe"
                      className="w-full px-4 py-2.5 rounded text-sm"
                      style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                    />
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium mb-2">Country</label>
                      <select
                        name="country"
                        value={formData.country}
                        onChange={handleChange}
                        required
                        className="w-full px-4 py-2.5 rounded text-sm"
                        style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                      >
                        <option value="US">United States</option>
                        <option value="CA">Canada</option>
                        <option value="GB">United Kingdom</option>
                        <option value="DE">Germany</option>
                        <option value="FR">France</option>
                        <option value="IN">India</option>
                      </select>
                    </div>
                    <div>
                      <label className="block text-sm font-medium mb-2">ZIP / Postal code</label>
                      <input
                        type="text"
                        name="zip"
                        value={formData.zip}
                        onChange={handleChange}
                        required
                        placeholder="10001"
                        className="w-full px-4 py-2.5 rounded text-sm"
                        style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                      />
                    </div>
                  </div>
                </div>
              </div>

              <div className="card p-6">
                <h2 className="text-sm font-semibold mb-4">Billing address</h2>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium mb-2">Company name</label>
                    <input
                      type="text"
                      placeholder="Acme Inc."
                      className="w-full px-4 py-2.5 rounded text-sm"
                      style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium mb-2">Address</label>
                    <input
                      type="text"
                      placeholder="123 Main St"
                      className="w-full px-4 py-2.5 rounded text-sm"
                      style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                    />
                  </div>
                </div>
              </div>

              <button
                type="submit"
                disabled={processing}
                className="group btn-primary w-full py-3 text-sm flex items-center justify-center gap-2"
              >
                {processing ? (
                  <>Processing...</>
                ) : (
                  <>
                    <Lock size={14} />
                    Pay $500.00 and subscribe
                    <ArrowRight size={14} className="transition-transform group-hover:translate-x-0.5" />
                  </>
                )}
              </button>

              <p className="text-xs text-center" style={{ color: 'var(--text-muted)' }}>
                By subscribing, you agree to our{' '}
                <a href="/terms" className="underline">Terms of Service</a>
                {' '}and{' '}
                <a href="/privacy" className="underline">Privacy Policy</a>
              </p>
            </form>
          </div>

          {/* Order Summary */}
          <div className="md:col-span-2">
            <div className="card p-6 sticky top-20">
              <h2 className="text-sm font-semibold mb-4">Order summary</h2>
              
              <div className="space-y-3 mb-6">
                <div className="flex justify-between text-sm">
                  <span style={{ color: 'var(--text-secondary)' }}>Growth plan</span>
                  <span className="font-mono">$500.00</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span style={{ color: 'var(--text-secondary)' }}>Billing cycle</span>
                  <span>Monthly</span>
                </div>
              </div>

              <div className="pt-4 border-t mb-6" style={{ borderColor: 'var(--border-primary)' }}>
                <div className="flex justify-between items-baseline">
                  <span className="font-semibold">Total due today</span>
                  <span className="text-2xl font-bold font-mono">$500</span>
                </div>
                <p className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
                  + applicable taxes
                </p>
              </div>

              <div className="space-y-2 text-xs" style={{ color: 'var(--text-muted)' }}>
                <p>✓ 14-day free trial</p>
                <p>✓ Cancel anytime</p>
                <p>✓ 30-day money-back guarantee</p>
              </div>

              <div className="mt-6 pt-6 border-t" style={{ borderColor: 'var(--border-primary)' }}>
                <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
                  Secure payment processed by Stripe. Your card information is encrypted and never stored on our servers.
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
