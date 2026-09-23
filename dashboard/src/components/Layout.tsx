import { useState, useEffect, useMemo, useCallback } from 'react';
import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { ChevronDown } from 'lucide-react';
import { useTheme } from '../context/ThemeContext';
import { listDomains, type Domain } from '../lib/api';
import { supabase } from '../lib/supabaseClient';
import type { User } from '@supabase/supabase-js';

const tabs = [
  { path: '/', label: 'Overview' },
  { path: '/evidence-logs', label: 'Evidence' },
  { path: '/mitigation-rules', label: 'Rules' },
  { path: '/protection-settings', label: 'Settings' },
  { path: '/domains-siem', label: 'Domains' },
];

export interface LayoutContext {
  domains: Domain[];
  selectedDomain: Domain | null;
  domainsLoading: boolean;
  onDomainAdded: (domain: Domain) => void;
}

export default function Layout() {
  const [tenantOpen, setTenantOpen] = useState(false);
  const [userOpen, setUserOpen] = useState(false);
  const [domains, setDomains] = useState<Domain[]>([]);
  const [domainsLoading, setDomainsLoading] = useState(true);
  const [selectedDomain, setSelectedDomain] = useState<Domain | null>(null);
  const [user, setUser] = useState<User | null>(null);
  const { theme, toggleTheme } = useTheme();
  const navigate = useNavigate();

  useEffect(() => {
    let cancelled = false;
    listDomains()
      .then((d) => {
        if (cancelled) return;
        setDomains(d);
        setSelectedDomain(d[0] ?? null);
      })
      .catch(() => {})
      .finally(() => !cancelled && setDomainsLoading(false));
    supabase.auth.getUser().then(({ data }) => !cancelled && setUser(data.user)).catch(() => {});
    return () => {
      cancelled = true;
    };
  }, []);

  const handleDomainAdded = useCallback((domain: Domain) => {
    setDomains((prev) => [domain, ...prev]);
    setSelectedDomain((prev) => prev ?? domain);
  }, []);

  const contextValue = useMemo<LayoutContext>(() => ({
    domains,
    selectedDomain,
    domainsLoading,
    onDomainAdded: handleDomainAdded,
  }), [domains, selectedDomain, domainsLoading, handleDomainAdded]);

  const handleSignOut = async () => {
    await supabase.auth.signOut();
    navigate('/sign-in');
  };

  const displayName = (user?.user_metadata?.name as string | undefined) || user?.email || '';
  const initials = displayName
    ? displayName.split(' ').map((p) => p[0]).slice(0, 2).join('').toUpperCase()
    : '..';

  return (
    <div className="min-h-screen" style={{ background: 'var(--bg-primary)' }}>
      {/* Top Navigation */}
      <header className="sticky top-0 z-50 border-b" style={{ borderColor: 'var(--border-primary)', background: theme === 'dark' ? 'rgba(0,0,0,0.8)' : 'rgba(255,255,255,0.8)', backdropFilter: 'blur(12px)' }}>
        <div className="max-w-[1400px] mx-auto px-4 h-12 flex items-center justify-between">
          {/* Left */}
          <div className="flex items-center gap-5">
            <a href="/landing" className="font-bold text-sm tracking-tight" style={{ color: 'var(--text-primary)' }}>hakaishield</a>

            {/* Domain Switcher */}
            <div className="relative">
              <button
                onClick={() => setTenantOpen(!tenantOpen)}
                className="flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium transition-colors"
                style={{ border: '1px solid var(--border-secondary)', color: 'var(--text-secondary)' }}
              >
                <span>{domainsLoading ? 'Loading…' : selectedDomain?.domain ?? 'No domains yet'}</span>
                <ChevronDown size={12} style={{ color: 'var(--text-muted)' }} />
              </button>
              {tenantOpen && (
                <div className="absolute top-full left-0 mt-1 w-56 rounded-lg py-1 z-50" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-secondary)' }}>
                  {domains.length === 0 && (
                    <div className="px-3 py-2 text-xs" style={{ color: 'var(--text-muted)' }}>
                      No domains added yet.{' '}
                      <NavLink to="/domains-siem" className="underline" onClick={() => setTenantOpen(false)}>Add one</NavLink>
                    </div>
                  )}
                  {domains.map((d) => (
                    <button
                      key={d.id}
                      onClick={() => { setSelectedDomain(d); setTenantOpen(false); }}
                      className="w-full text-left px-3 py-1.5 text-xs transition-colors flex items-center justify-between"
                      style={{ color: d.id === selectedDomain?.id ? 'var(--text-primary)' : 'var(--text-secondary)' }}
                    >
                      <span>{d.domain}</span>
                      <span className={`badge ${d.status === 'active' ? 'badge-green' : 'badge-yellow'}`}>
                        {d.status}
                      </span>
                    </button>
                  ))}
                </div>
              )}
            </div>

            {/* Nav Tabs */}
            <nav className="hidden md:flex items-center gap-0.5">
              {tabs.map(tab => (
                <NavLink
                  key={tab.path}
                  to={tab.path}
                  className={({ isActive }) => `nav-tab ${isActive ? 'active' : ''}`}
                  end={tab.path === '/'}
                >
                  {tab.label}
                </NavLink>
              ))}
            </nav>
          </div>

          {/* Right */}
          <div className="flex items-center gap-3">
            <a href="/landing" className="hidden sm:block text-xs" style={{ color: 'var(--text-muted)' }}>
              Public site
            </a>
            <button onClick={toggleTheme} className="text-xs" style={{ color: 'var(--text-muted)' }}>
              {theme === 'dark' ? 'Light' : 'Dark'}
            </button>
            <div className="relative">
              <button
                onClick={() => setUserOpen(!userOpen)}
                className="w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-medium text-white"
                style={{ background: 'var(--text-muted)' }}
                title={user?.email}
              >
                {initials}
              </button>
              {userOpen && (
                <div className="absolute top-full right-0 mt-1 w-44 rounded-lg py-1 z-50" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-secondary)' }}>
                  {user && (
                    <div className="px-3 py-2 text-xs truncate" style={{ color: 'var(--text-muted)', borderBottom: '1px solid var(--border-secondary)' }}>
                      {user.email}
                    </div>
                  )}
                  <button
                    onClick={handleSignOut}
                    className="w-full text-left px-3 py-1.5 text-xs transition-colors"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    Sign out
                  </button>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Mobile Nav */}
        <div className="md:hidden border-t px-2 py-1 flex gap-0.5 overflow-x-auto" style={{ borderColor: 'var(--border-primary)' }}>
          {tabs.map(tab => (
            <NavLink
              key={tab.path}
              to={tab.path}
              className={({ isActive }) => `nav-tab whitespace-nowrap text-xs ${isActive ? 'active' : ''}`}
              end={tab.path === '/'}
            >
              {tab.label}
            </NavLink>
          ))}
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-[1400px] mx-auto px-4 py-6">
        <Outlet context={contextValue} />
      </main>
    </div>
  );
}
