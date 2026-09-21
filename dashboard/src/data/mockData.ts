// Realistic mock data for HakaiShield dashboard

export const ja4Hashes = [
  't13d1516h2_8daaf6152771_0271d189196b',
  't13d1515h2_5b3720a8116e_b186b90428d3',
  't13d9816h2_c60e45e3a45b_4e7f8a2c1d09',
  't12d1516h2_7f3c8e2a1b4d_9e2f1a3b5c7d',
  't13d1516h3_a1b2c3d4e5f6_1a2b3c4d5e6f',
  't13d1515h2_2f8e9c3a7b1d_d4e5f6a7b8c9',
  't13d1516h2_f6e5d4c3b2a1_9f8e7d6c5b4a',
  't12d1515h2_1a2b3c4d5e6f_a1b2c3d4e5f6',
];

export const ips = [
  '185.220.101.42',
  '45.155.205.233',
  '194.26.29.120',
  '23.129.64.210',
  '104.244.76.13',
  '91.219.236.88',
  '178.128.23.49',
  '103.152.220.73',
  '172.67.182.31',
  '198.51.100.14',
];

export const asns = [
  'AS9009 M247 Ltd',
  'AS212238 Datacamp Limited',
  'AS48693 RUVDS-BGP',
  'AS396507 Tor Exit Node',
  'AS13335 Cloudflare Inc',
  'AS47583 HOSTJOY',
  'AS14061 DigitalOcean',
  'AS138995 Knownsec',
];

export const paths = [
  '/api/v1/users',
  '/api/v1/products',
  '/api/v1/auth/login',
  '/api/v2/search',
  '/api/v1/orders',
  '/graphql',
  '/api/v1/inventory',
  '/api/v1/pricing',
  '/wp-admin',
  '/.env',
  '/api/v1/payments',
  '/robots.txt',
  '/sitemap.xml',
  '/api/v1/reviews',
  '/api/v1/checkout',
];

export const methods = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'];

export const signals = [
  'UA Mismatch',
  'TLS Fragmentation',
  'HTTP/2 Anomaly',
  'Missing Headers',
  'JA4 Known Bot',
  'Request Velocity',
  'Header Order',
  'Cipher Suite Mismatch',
  'SNI Mismatch',
  'Empty TLS Extensions',
  'Non-Browser TLS',
  'Headless Signature',
];

export const decisions = ['PASS', 'BLOCK', 'CHALLENGE', 'DECEIVE'] as const;
export type Decision = typeof decisions[number];

export const geos = [
  'US', 'DE', 'RU', 'CN', 'NL', 'FR', 'GB', 'SG', 'BR', 'IN', 'UA', 'RO',
];

// Generate traffic volume data for last 24h
export function generateTrafficData() {
  const data = [];
  const now = new Date();
  for (let i = 23; i >= 0; i--) {
    const hour = new Date(now);
    hour.setHours(now.getHours() - i);
    const baseTraffic = 8000 + Math.random() * 4000;
    const attackMultiplier = i > 8 && i < 16 ? 1.5 : 1;
    data.push({
      time: `${hour.getHours().toString().padStart(2, '0')}:00`,
      passed: Math.round(baseTraffic * 0.82 * attackMultiplier),
      blocked: Math.round(baseTraffic * 0.08 * attackMultiplier),
      challenged: Math.round(baseTraffic * 0.06 * attackMultiplier),
      deceived: Math.round(baseTraffic * 0.04 * attackMultiplier),
    });
  }
  return data;
}

// Generate evidence logs
export function generateLogs(count: number) {
  const logs = [];
  const now = Date.now();
  for (let i = 0; i < count; i++) {
    const decision = decisions[Math.floor(Math.random() * decisions.length)];
    const score = decision === 'PASS'
      ? Math.floor(Math.random() * 30)
      : decision === 'CHALLENGE'
      ? 40 + Math.floor(Math.random() * 30)
      : decision === 'DECEIVE'
      ? 60 + Math.floor(Math.random() * 20)
      : 80 + Math.floor(Math.random() * 20);

    const numSignals = decision === 'PASS' ? Math.floor(Math.random() * 2) : 1 + Math.floor(Math.random() * 4);
    const logSignals: string[] = [];
    for (let j = 0; j < numSignals; j++) {
      const sig = signals[Math.floor(Math.random() * signals.length)];
      if (!logSignals.includes(sig)) logSignals.push(sig);
    }

    logs.push({
      id: `log-${i}`,
      timestamp: new Date(now - i * (15000 + Math.random() * 30000)).toISOString(),
      method: methods[Math.floor(Math.random() * methods.length)],
      path: paths[Math.floor(Math.random() * paths.length)],
      ip: ips[Math.floor(Math.random() * ips.length)],
      geo: geos[Math.floor(Math.random() * geos.length)],
      ja4: ja4Hashes[Math.floor(Math.random() * ja4Hashes.length)],
      signals: logSignals,
      score,
      decision,
      headers: {
        'User-Agent': logSignals.includes('UA Mismatch') ? 'python-requests/2.28.1' : 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',
        'Accept': 'text/html,application/xhtml+xml',
        'Accept-Language': 'en-US,en;q=0.9',
        'Accept-Encoding': 'gzip, deflate, br',
        'Connection': 'keep-alive',
        'X-Forwarded-For': ips[Math.floor(Math.random() * ips.length)],
        'X-Real-IP': ips[Math.floor(Math.random() * ips.length)],
      },
      tls: {
        version: 'TLS 1.3',
        cipher: 'TLS_AES_256_GCM_SHA384',
        sni: 'api.example.com',
        alpn: 'h2',
        extensions: 'server_name,extended_master_status,session_ticket,supported_groups,ec_point_formats,sig_algs,psk_key_exchange_modes,key_share',
      },
    });
  }
  return logs;
}

// Top offenders
export const topOffenders = [
  { ip: '185.220.101.42', ja4: ja4Hashes[0], asn: asns[0], geo: 'DE', blocked: 12847, lastSeen: '2 min ago' },
  { ip: '45.155.205.233', ja4: ja4Hashes[1], asn: asns[1], geo: 'RU', blocked: 9234, lastSeen: '5 min ago' },
  { ip: '194.26.29.120', ja4: ja4Hashes[2], asn: asns[2], geo: 'NL', blocked: 7891, lastSeen: '1 min ago' },
  { ip: '23.129.64.210', ja4: ja4Hashes[3], asn: asns[3], geo: 'US', blocked: 6452, lastSeen: '8 min ago' },
  { ip: '104.244.76.13', ja4: ja4Hashes[4], asn: asns[4], geo: 'US', blocked: 5103, lastSeen: '3 min ago' },
  { ip: '91.219.236.88', ja4: ja4Hashes[5], asn: asns[5], geo: 'RO', blocked: 4892, lastSeen: '12 min ago' },
  { ip: '178.128.23.49', ja4: ja4Hashes[6], asn: asns[6], geo: 'SG', blocked: 3761, lastSeen: '6 min ago' },
  { ip: '103.152.220.73', ja4: ja4Hashes[7], asn: asns[7], geo: 'CN', blocked: 2984, lastSeen: '15 min ago' },
];

// Managed rules
export const managedRules = [
  { id: 'mr-1', name: 'Block Known Scrapers', description: 'Blocks requests from known scraping frameworks and libraries', enabled: true, hits: 23456 },
  { id: 'mr-2', name: 'Block Headless Browsers', description: 'Detects and blocks Puppeteer, Playwright, Selenium signatures', enabled: true, hits: 18234 },
  { id: 'mr-3', name: 'Rate Limit Aggressive IPs', description: 'Limits requests from IPs exceeding 100 req/min threshold', enabled: true, hits: 9812 },
  { id: 'mr-4', name: 'Block TOR Exit Nodes', description: 'Blocks traffic from known Tor exit node IP ranges', enabled: false, hits: 4521 },
  { id: 'mr-5', name: 'Challenge Suspicious Bots', description: 'Issues CAPTCHA challenges to bots with threat score > 50', enabled: true, hits: 15678 },
  { id: 'mr-6', name: 'Block Data Center IPs', description: 'Blocks requests from known hosting provider IP ranges', enabled: false, hits: 7234 },
  { id: 'mr-7', name: 'TLS Fingerprint Enforcement', description: 'Validates TLS fingerprints against known browser profiles', enabled: true, hits: 11245 },
  { id: 'mr-8', name: 'HTTP/2 Protocol Validation', description: 'Ensures proper HTTP/2 frame ordering and settings', enabled: true, hits: 6789 },
];

// Custom exceptions (whitelisted)
export const exceptions = [
  { id: 'ex-1', type: 'IP', value: '172.67.182.31', reason: 'Office IP - Engineering Team', createdAt: '2024-01-15' },
  { id: 'ex-2', type: 'Bot', value: 'Googlebot', reason: 'Official Google crawler', createdAt: '2024-01-10' },
  { id: 'ex-3', type: 'IP', value: '198.51.100.14', reason: 'Monitoring service - Datadog', createdAt: '2024-02-01' },
  { id: 'ex-4', type: 'Bot', value: 'Bingbot', reason: 'Official Bing crawler', createdAt: '2024-01-10' },
  { id: 'ex-5', type: 'JA4', value: 't13d1516h2_8daaf6152771_0271d189196b', reason: 'Internal service fingerprint', createdAt: '2024-02-10' },
];

// Protected domains
export const domains = [
  { id: 'd-1', domain: 'api.example.com', origin: '10.0.1.50:8080', certStatus: 'valid', certExpiry: '2025-08-15', status: 'active', requests: '2.4M/day' },
  { id: 'd-2', domain: 'www.example.com', origin: '10.0.1.51:3000', certStatus: 'valid', certExpiry: '2025-06-20', status: 'active', requests: '890K/day' },
  { id: 'd-3', domain: 'staging.example.com', origin: '10.0.2.10:8080', certStatus: 'expiring', certExpiry: '2025-02-28', status: 'active', requests: '45K/day' },
  { id: 'd-4', domain: 'admin.example.com', origin: '10.0.1.52:4000', certStatus: 'valid', certExpiry: '2025-09-01', status: 'active', requests: '12K/day' },
  { id: 'd-5', domain: 'cdn.example.com', origin: 's3.amazonaws.com', certStatus: 'valid', certExpiry: '2025-11-30', status: 'active', requests: '5.1M/day' },
];

// SIEM integrations
export const siemIntegrations = [
  { id: 'siem-1', name: 'Datadog', description: 'Stream logs and metrics to Datadog SIEM', status: 'connected', lastSync: '30s ago', events: '1.2M' },
  { id: 'siem-2', name: 'Splunk', description: 'Forward events to Splunk HTTP Event Collector', status: 'connected', lastSync: '1m ago', events: '890K' },
  { id: 'siem-3', name: 'AWS S3', description: 'Archive logs to S3 bucket in Parquet format', status: 'disconnected', lastSync: '2h ago', events: '45M' },
  { id: 'siem-4', name: 'Elastic Security', description: 'Ship logs to Elasticsearch cluster', status: 'disconnected', lastSync: 'Never', events: '0' },
];
