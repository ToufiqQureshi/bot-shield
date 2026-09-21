import { useState } from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import { ChevronDown, Sun, Moon } from 'lucide-react';
import { useTheme } from '../context/ThemeContext';

const tabs = [
  { path: '/', label: 'Overview' },
  { path: '/evidence-logs', label: 'Evidence' },
  { path: '/mitigation-rules', label: 'Rules' },
  { path: '/protection-settings', label: 'Settings' },
  { path: '/domains-siem', label: 'Domains' },
];

const tenants = [
  { id: '1', name: 'Production', env: 'prod' },
  { id: '2', name: 'Staging', env: 'staging' },
  { id: '3', name: 'Development', env: 'dev' },
];

export default function Layout() {
  const [tenantOpen, setTenantOpen] = useState(false);
  const [selectedTenant, setSelectedTenant] = useState(tenants[0]);
  const { theme, toggleTheme } = useTheme();

  return (
    <div className="min-h-screen" style={{ background: 'var(--bg-primary)' }}>
      {/* Top Navigation */}
      <header className="sticky top-0 z-50 border-b" style={{ borderColor: 'var(--border-primary)', background: theme === 'dark' ? 'rgba(0,0,0,0.8)' : 'rgba(255,255,255,0.8)', backdropFilter: 'blur(12px)' }}>
        <div className="max-w-[1400px] mx-auto px-4 h-12 flex items-center justify-between">
          {/* Left */}
          <div className="flex items-center gap-5">
            <a href="/landing" className="font-bold text-sm tracking-tight" style={{ color: 'var(--text-primary)' }}>hakaishield</a>

            {/* Tenant Switcher */}
            <div className="relative">
              <button
                onClick={() => setTenantOpen(!tenantOpen)}
                className="flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium transition-colors"
                style={{ border: '1px solid var(--border-secondary)', color: 'var(--text-secondary)' }}
              >
                <span>{selectedTenant.name}</span>
                <ChevronDown size={12} style={{ color: 'var(--text-muted)' }} />
              </button>
              {tenantOpen && (
                <div className="absolute top-full left-0 mt-1 w-48 rounded-lg py-1 z-50" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-secondary)' }}>
                  {tenants.map(t => (
                    <button
                      key={t.id}
                      onClick={() => { setSelectedTenant(t); setTenantOpen(false); }}
                      className="w-full text-left px-3 py-1.5 text-xs transition-colors flex items-center justify-between"
                      style={{ color: t.id === selectedTenant.id ? 'var(--text-primary)' : 'var(--text-secondary)' }}
                    >
                      <span>{t.name}</span>
                      <span className={`badge ${t.env === 'prod' ? 'badge-green' : t.env === 'staging' ? 'badge-yellow' : 'badge-gray'}`}>
                        {t.env}
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
            <div className="w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-medium text-white" style={{ background: 'var(--text-muted)' }}>
              JD
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
        <Outlet />
      </main>
    </div>
  );
}
