import { useTheme } from '../context/ThemeContext';

export default function Privacy() {
  const { theme, toggleTheme } = useTheme();

  return (
    <div className="min-h-screen" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      {/* Nav */}
      <nav className="sticky top-0 z-50 border-b" style={{ borderColor: 'var(--border-primary)', background: theme === 'dark' ? 'rgba(0,0,0,0.8)' : 'rgba(255,255,255,0.8)', backdropFilter: 'blur(12px)' }}>
        <div className="max-w-4xl mx-auto px-6 h-14 flex items-center justify-between">
          <a href="/landing" className="font-bold text-sm tracking-tight">hakaishield</a>
          <div className="flex items-center gap-4">
            <button onClick={toggleTheme} className="text-sm" style={{ color: 'var(--text-muted)' }}>
              {theme === 'dark' ? 'Light' : 'Dark'}
            </button>
            <a href="/landing" className="text-sm" style={{ color: 'var(--text-secondary)' }}>Home</a>
          </div>
        </div>
      </nav>

      <div className="max-w-4xl mx-auto px-6 py-16">
        <p className="text-sm font-mono mb-4" style={{ color: 'var(--text-muted)' }}>
          // legal
        </p>
        <h1 className="text-4xl font-bold mb-2 tracking-tight">Privacy Policy</h1>
        <p className="text-sm mb-12" style={{ color: 'var(--text-muted)' }}>Last updated: September 17, 2026</p>

        <div className="prose max-w-none space-y-6 text-base leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
          <p>
            This Privacy Policy describes how hakaishield ("we", "us", or "our") collects, uses, and shares information when you use our services.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>1. Information We Collect</h2>
          
          <h3 className="text-xl font-semibold mt-6 mb-3" style={{ color: 'var(--text-primary)' }}>1.1 Traffic Data</h3>
          <p>
            When traffic passes through our Service, we process the following information in-memory:
          </p>
          <ul className="list-disc list-inside space-y-2 ml-4">
            <li>IP addresses</li>
            <li>TLS handshake data (JA4 fingerprints)</li>
            <li>HTTP headers (User-Agent, Accept, etc.)</li>
            <li>Request metadata (path, method, timestamp)</li>
          </ul>
          <p>
            <strong style={{ color: 'var(--text-primary)' }}>We do not log request bodies or payloads.</strong>
          </p>

          <h3 className="text-xl font-semibold mt-6 mb-3" style={{ color: 'var(--text-primary)' }}>1.2 Account Information</h3>
          <p>
            When you create an account, we collect:
          </p>
          <ul className="list-disc list-inside space-y-2 ml-4">
            <li>Name and email address</li>
            <li>Company name (optional)</li>
            <li>Payment information (processed by Stripe)</li>
          </ul>

          <h3 className="text-xl font-semibold mt-6 mb-3" style={{ color: 'var(--text-primary)' }}>1.3 Usage Data</h3>
          <p>
            We collect usage data including:
          </p>
          <ul className="list-disc list-inside space-y-2 ml-4">
            <li>Request volume and patterns</li>
            <li>Scoring decisions and evidence trails</li>
            <li>Feature usage</li>
            <li>Error logs</li>
          </ul>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>2. How We Use Information</h2>
          <p>
            We use the information we collect to:
          </p>
          <ul className="list-disc list-inside space-y-2 ml-4">
            <li>Provide and maintain the Service</li>
            <li>Score and make decisions about incoming traffic</li>
            <li>Generate evidence trails for transparency</li>
            <li>Improve our scoring algorithms</li>
            <li>Communicate with you about your account</li>
            <li>Process payments</li>
            <li>Comply with legal obligations</li>
          </ul>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>3. Data Retention</h2>
          <p>
            We retain data as follows:
          </p>
          <ul className="list-disc list-inside space-y-2 ml-4">
            <li><strong style={{ color: 'var(--text-primary)' }}>Traffic data:</strong> Processed in-memory, retained for 24 hours for evidence trails (Starter plan) or 30 days (Growth plan)</li>
            <li><strong style={{ color: 'var(--text-primary)' }}>Account data:</strong> Retained for the duration of your account plus 30 days after termination</li>
            <li><strong style={{ color: 'var(--text-primary)' }}>Payment data:</strong> Retained as required by law and payment processors</li>
          </ul>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>4. Data Sharing</h2>
          <p>
            We do not sell your data. We share data only in the following circumstances:
          </p>
          <ul className="list-disc list-inside space-y-2 ml-4">
            <li><strong style={{ color: 'var(--text-primary)' }}>Service providers:</strong> We share data with Stripe for payment processing</li>
            <li><strong style={{ color: 'var(--text-primary)' }}>Legal requirements:</strong> We may disclose data if required by law or to protect our rights</li>
            <li><strong style={{ color: 'var(--text-primary)' }}>Business transfers:</strong> In the event of a merger or acquisition, your data may be transferred</li>
          </ul>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>5. Data Security</h2>
          <p>
            We implement appropriate technical and organizational measures to protect your data, including:
          </p>
          <ul className="list-disc list-inside space-y-2 ml-4">
            <li>Encryption in transit (TLS 1.3)</li>
            <li>Encryption at rest for stored data</li>
            <li>Access controls and authentication</li>
            <li>Regular security audits</li>
          </ul>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>6. Your Rights</h2>
          <p>
            Depending on your location, you may have the following rights:
          </p>
          <ul className="list-disc list-inside space-y-2 ml-4">
            <li>Access your personal data</li>
            <li>Correct inaccurate data</li>
            <li>Delete your data</li>
            <li>Restrict or object to processing</li>
            <li>Data portability</li>
            <li>Withdraw consent</li>
          </ul>
          <p>
            To exercise these rights, please contact us at <a href="mailto:privacy@hakaishield.io" style={{ color: 'var(--accent-blue)' }}>privacy@hakaishield.io</a>.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>7. International Transfers</h2>
          <p>
            Your data may be transferred to and processed in countries other than your own. We ensure appropriate safeguards are in place for such transfers, including Standard Contractual Clauses where required.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>8. Children's Privacy</h2>
          <p>
            The Service is not intended for use by children under 16. We do not knowingly collect personal information from children.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>9. Changes to This Policy</h2>
          <p>
            We may update this Privacy Policy from time to time. We will notify you of material changes via email or through the Service.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>10. Contact Us</h2>
          <p>
            If you have questions about this Privacy Policy, please contact us at:
          </p>
          <p>
            Email: <a href="mailto:privacy@hakaishield.io" style={{ color: 'var(--accent-blue)' }}>privacy@hakaishield.io</a>
          </p>
        </div>
      </div>
    </div>
  );
}
