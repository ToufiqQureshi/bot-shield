import { useState } from 'react';
import { useTheme } from '../context/ThemeContext';
import { ArrowRight } from 'lucide-react';

export default function SignUp() {
  const { theme, toggleTheme } = useTheme();
  const [formData, setFormData] = useState({
    name: '',
    email: '',
    password: '',
    company: '',
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    // In production, this would create account
    console.log('Sign up:', formData);
    // Redirect to onboarding
    window.location.href = '/onboarding';
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  return (
    <div className="min-h-screen flex items-center justify-center py-12" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="w-full max-w-md px-6">
        <div className="text-center mb-8">
          <a href="/landing" className="font-bold text-xl tracking-tight">hakaishield</a>
        </div>

        <div className="card p-8">
          <h1 className="text-2xl font-bold mb-2 tracking-tight">Create your account</h1>
          <p className="text-sm mb-8" style={{ color: 'var(--text-secondary)' }}>
            Start protecting your site in minutes
          </p>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-medium mb-2">Full name</label>
              <input
                type="text"
                name="name"
                value={formData.name}
                onChange={handleChange}
                required
                className="w-full px-4 py-2.5 rounded text-sm"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                placeholder="John Doe"
              />
            </div>

            <div>
              <label className="block text-sm font-medium mb-2">Work email</label>
              <input
                type="email"
                name="email"
                value={formData.email}
                onChange={handleChange}
                required
                className="w-full px-4 py-2.5 rounded text-sm"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                placeholder="you@company.com"
              />
            </div>

            <div>
              <label className="block text-sm font-medium mb-2">Password</label>
              <input
                type="password"
                name="password"
                value={formData.password}
                onChange={handleChange}
                required
                minLength={8}
                className="w-full px-4 py-2.5 rounded text-sm"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                placeholder="At least 8 characters"
              />
            </div>

            <div>
              <label className="block text-sm font-medium mb-2">Company name (optional)</label>
              <input
                type="text"
                name="company"
                value={formData.company}
                onChange={handleChange}
                className="w-full px-4 py-2.5 rounded text-sm"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                placeholder="Acme Inc."
              />
            </div>

            <div className="flex items-start gap-2 pt-2">
              <input type="checkbox" id="terms" required className="mt-1 rounded" />
              <label htmlFor="terms" className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                I agree to the{' '}
                <a href="/terms" className="underline" style={{ color: 'var(--accent-blue)' }}>Terms of Service</a>
                {' '}and{' '}
                <a href="/privacy" className="underline" style={{ color: 'var(--accent-blue)' }}>Privacy Policy</a>
              </label>
            </div>

            <button type="submit" className="group btn-primary w-full py-2.5 text-sm flex items-center justify-center gap-2">
              Create account
              <ArrowRight size={14} className="transition-transform group-hover:translate-x-0.5" />
            </button>
          </form>

          <div className="mt-6 pt-6 border-t text-center" style={{ borderColor: 'var(--border-primary)' }}>
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              Already have an account?{' '}
              <a href="/sign-in" className="font-medium" style={{ color: 'var(--accent-blue)' }}>Sign in</a>
            </p>
          </div>
        </div>

        <div className="mt-6 text-center">
          <button onClick={toggleTheme} className="text-sm" style={{ color: 'var(--text-muted)' }}>
            {theme === 'dark' ? 'Light mode' : 'Dark mode'}
          </button>
        </div>
      </div>
    </div>
  );
}
