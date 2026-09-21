import { useMemo } from 'react';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts';
import { generateTrafficData, topOffenders } from '../data/mockData';

const metrics = [
  { label: 'Total Requests', value: '2.41M', change: '+12.3%', up: true },
  { label: 'Clean Traffic', value: '1.97M', change: '+8.1%', up: true },
  { label: 'Blocked', value: '187K', change: '+23.4%', up: false, color: 'red' },
  { label: 'Challenged', value: '142K', change: '-5.2%', up: true, color: 'yellow' },
  { label: 'Deceived', value: '98K', change: '+31.7%', up: true, color: 'orange' },
];

function CustomTooltip({ active, payload, label }: any) {
  if (!active || !payload) return null;
  return (
    <div className="custom-tooltip">
      <p className="font-medium mb-1.5" style={{ color: 'var(--text-primary)' }}>{label}</p>
      {payload.map((entry: any, i: number) => (
        <div key={i} className="flex items-center gap-2 text-xs">
          <span className="w-2 h-2 rounded-full" style={{ background: entry.color }}></span>
          <span style={{ color: 'var(--text-secondary)' }}>{entry.name}:</span>
          <span className="font-mono" style={{ color: 'var(--text-primary)' }}>{entry.value.toLocaleString()}</span>
        </div>
      ))}
    </div>
  );
}

export default function Overview() {
  const trafficData = useMemo(() => generateTrafficData(), []);

  return (
    <div className="space-y-6 animate-in">
      {/* Shadow Mode Banner */}
      <div className="flex items-center gap-3 px-4 py-3 rounded-lg" style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-secondary)' }}>
        <span className="w-2 h-2 rounded-full bg-yellow-500 animate-pulse flex-shrink-0"></span>
        <div className="flex-1 flex items-center gap-2">
          <span className="text-sm font-semibold text-yellow-500">Shadow Mode</span>
          <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>Scoring traffic but not blocking. <button className="text-yellow-500 hover:text-yellow-400 font-medium underline underline-offset-2">Switch to Active →</button></span>
        </div>
        <span className="badge badge-yellow text-[10px] hidden sm:inline-flex">SAFE MODE</span>
      </div>

      {/* Metrics Strip */}
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-3">
        {metrics.map((m, i) => (
          <div key={i} className="card p-4">
            <div className="flex items-center justify-between mb-2">
              <p className="text-xs" style={{ color: 'var(--text-muted)' }}>{m.label}</p>
              <span className={`text-xs font-mono ${m.up ? 'text-green-400' : 'text-red-400'}`}>
                {m.change}
              </span>
            </div>
            <p className="text-2xl font-bold font-mono" style={{ color: 'var(--text-primary)' }}>{m.value}</p>
          </div>
        ))}
      </div>

      {/* Traffic Chart */}
      <div className="card p-5">
        <div className="flex items-center justify-between mb-4">
          <div>
            <p className="text-xs font-mono mb-1" style={{ color: 'var(--text-muted)' }}>// traffic</p>
            <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Last 24 hours</h2>
          </div>
          <div className="flex gap-1">
            <button className="btn-secondary text-xs py-1">24h</button>
            <button className="nav-tab text-xs py-1">7d</button>
            <button className="nav-tab text-xs py-1">30d</button>
          </div>
        </div>
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={trafficData} margin={{ top: 5, right: 5, left: -10, bottom: 0 }}>
              <defs>
                <linearGradient id="colorPassed" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#22c55e" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="colorBlocked" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#ef4444" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#ef4444" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="colorChallenged" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#eab308" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#eab308" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="colorDeceived" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#f97316" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#f97316" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="#1f1f1f" vertical={false} />
              <XAxis dataKey="time" tick={{ fontSize: 11, fill: '#71717a' }} axisLine={{ stroke: '#1f1f1f' }} tickLine={false} />
              <YAxis tick={{ fontSize: 11, fill: '#71717a' }} axisLine={false} tickLine={false} tickFormatter={(v) => `${(v/1000).toFixed(0)}k`} />
              <Tooltip content={<CustomTooltip />} />
              <Legend iconType="circle" iconSize={8} wrapperStyle={{ fontSize: '11px', paddingTop: '8px' }} />
              <Area type="monotone" dataKey="passed" name="Passed" stroke="#22c55e" fill="url(#colorPassed)" strokeWidth={1.5} />
              <Area type="monotone" dataKey="blocked" name="Blocked" stroke="#ef4444" fill="url(#colorBlocked)" strokeWidth={1.5} />
              <Area type="monotone" dataKey="challenged" name="Challenged" stroke="#eab308" fill="url(#colorChallenged)" strokeWidth={1.5} />
              <Area type="monotone" dataKey="deceived" name="Deceived" stroke="#f97316" fill="url(#colorDeceived)" strokeWidth={1.5} />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Top Offenders */}
      <div className="card overflow-hidden">
        <div className="flex items-center justify-between px-5 py-4 border-b" style={{ borderColor: 'var(--border-primary)' }}>
          <div>
            <p className="text-xs font-mono mb-1" style={{ color: 'var(--text-muted)' }}>// top offenders</p>
            <h2 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Most blocked IPs</h2>
          </div>
          <button className="btn-secondary text-xs">View all</button>
        </div>
        <div className="overflow-x-auto">
          <table className="data-table">
            <thead>
              <tr>
                <th>IP Address</th>
                <th>JA4 Fingerprint</th>
                <th>ASN</th>
                <th>Geo</th>
                <th>Blocked</th>
                <th>Last Seen</th>
              </tr>
            </thead>
            <tbody>
              {topOffenders.map((o, i) => (
                <tr key={i}>
                  <td className="font-mono text-xs" style={{ color: 'var(--text-primary)' }}>{o.ip}</td>
                  <td className="font-mono text-xs" style={{ maxWidth: '200px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{o.ja4}</td>
                  <td className="text-xs">{o.asn}</td>
                  <td>
                    <span className="badge badge-gray">{o.geo}</span>
                  </td>
                  <td className="font-mono text-xs text-red-400">{o.blocked.toLocaleString()}</td>
                  <td className="text-xs" style={{ color: 'var(--text-muted)' }}>{o.lastSeen}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
