import { useState } from 'react';
import { useTheme } from '../context/ThemeContext';
import { ArrowRight, Sun, Moon } from 'lucide-react';

export default function SignIn() {
  const { theme, toggleTheme } = useTheme();
  const [formData, setFormData] = useState({
    email: '',
    password: '',
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    // In production, this would authenticate
    console.log('Sign in:', formData);
    // Redirect to dashboard
    window.location.href = '/';
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  return (
    <div className="min-h-screen flex items-center justify-center" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="w-full max-w-md px-6">
        <div className="text-center mb-8">
          <a href="/landing" className="font-bold text-xl tracking-tight">hakaishield</a>
        </div>

        <div className="card p-8">
          <h1 className="text-2xl font-bold mb-2 tracking-tight">Welcome back</h1>
          <p className="text-sm mb-8" style={{ color: 'var(--text-secondary)' }}>
            Sign in to your account to continue
          </p>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-medium mb-2">Email</label>
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
              <div className="flex items-center justify-between mb-2">
                <label className="block text-sm font-medium">Password</label>
                <a href="/forgot-password" className="text-xs" style={{ color: 'var(--accent-blue)' }}>Forgot password?</a>
              </div>
              <input
                type="password"
                name="password"
                value={formData.password}
                onChange={handleChange}
                required
                className="w-full px-4 py-2.5 rounded text-sm"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-primary)', color: 'var(--text-primary)' }}
                placeholder="••••••••"
              />
            </div>

            <div className="flex items-center gap-2">
              <input type="checkbox" id="remember" className="rounded" />
              <label htmlFor="remember" className="text-sm" style={{ color: 'var(--text-secondary)' }}>Remember me</label>
            </div>

            <button type="submit" className="group btn-primary w-full py-2.5 text-sm flex items-center justify-center gap-2">
              Sign in
              <ArrowRight size={14} className="transition-transform group-hover:translate-x-0.5" />
            </button>
          </form>

          <div className="mt-6 pt-6 border-t text-center" style={{ borderColor: 'var(--border-primary)' }}>
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              Don't have an account?{' '}
              <a href="/sign-up" className="font-medium" style={{ color: 'var(--accent-blue)' }}>Sign up</a>
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
