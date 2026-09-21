import { useTheme } from '../context/ThemeContext';

export default function Terms() {
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
        <h1 className="text-4xl font-bold mb-2 tracking-tight">Terms of Service</h1>
        <p className="text-sm mb-12" style={{ color: 'var(--text-muted)' }}>Last updated: September 17, 2026</p>

        <div className="prose max-w-none space-y-6 text-base leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
          <p>
            These Terms of Service ("Terms") govern your access to and use of hakaishield's website, products, and services (the "Service"). Please read these Terms carefully.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>1. Acceptance of Terms</h2>
          <p>
            By accessing or using the Service, you agree to be bound by these Terms. If you do not agree to these Terms, you may not access or use the Service.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>2. Description of Service</h2>
          <p>
            hakaishield provides inline bot protection and API security services. The Service analyzes incoming traffic, scores requests based on various signals, and makes decisions to allow, challenge, or block requests based on configurable thresholds.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>3. Account Registration</h2>
          <p>
            To access certain features of the Service, you may be required to create an account. You agree to provide accurate, current, and complete information during the registration process and to update such information to keep it accurate, current, and complete.
          </p>
          <p>
            You are responsible for safeguarding your account credentials and for all activities that occur under your account.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>4. Payment and Billing</h2>
          <p>
            The Service is offered on a subscription basis. You agree to pay all fees associated with your selected plan. Fees are billed in advance on a monthly or annual basis, depending on your selected billing cycle.
          </p>
          <p>
            All fees are non-refundable except as expressly provided in these Terms or as required by applicable law. We offer a 30-day money-back guarantee for new subscriptions.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>5. Acceptable Use</h2>
          <p>
            You agree not to use the Service to:
          </p>
          <ul className="list-disc list-inside space-y-2 ml-4">
            <li>Violate any applicable laws or regulations</li>
            <li>Infringe on the intellectual property rights of others</li>
            <li>Distribute malware or other harmful code</li>
            <li>Attempt to gain unauthorized access to the Service or other systems</li>
            <li>Use the Service to facilitate bot activity or automated abuse</li>
          </ul>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>6. Data Processing</h2>
          <p>
            The Service processes traffic data in-memory to make scoring decisions. We do not log request bodies or retain data beyond what is necessary for scoring and evidence trails.
          </p>
          <p>
            By using the Service, you represent that you have the right to send the traffic through our platform and that you have obtained all necessary consents from your users.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>7. Service Level Agreement</h2>
          <p>
            We strive to maintain high availability of the Service. Our target uptime is 99.9% on a monthly basis. In the event of downtime exceeding our SLA, you may be eligible for service credits as outlined in our SLA documentation.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>8. Limitation of Liability</h2>
          <p>
            To the maximum extent permitted by law, hakaishield shall not be liable for any indirect, incidental, special, consequential, or punitive damages, including but not limited to loss of profits, data, or business opportunities, arising out of or in connection with your use of the Service.
          </p>
          <p>
            Our total liability for any claims arising out of or relating to these Terms or the Service shall not exceed the amounts paid by you to hakaishield during the twelve (12) months preceding the claim.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>9. Termination</h2>
          <p>
            You may terminate your account at any time by contacting us or through the account settings. We may terminate or suspend your account immediately if you violate these Terms.
          </p>
          <p>
            Upon termination, your right to use the Service will cease immediately. We will retain your data for 30 days before deletion, unless required by law to retain it longer.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>10. Changes to Terms</h2>
          <p>
            We reserve the right to modify these Terms at any time. We will provide notice of material changes via email or through the Service. Your continued use of the Service after such modifications constitutes your acceptance of the updated Terms.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>11. Governing Law</h2>
          <p>
            These Terms shall be governed by and construed in accordance with the laws of the jurisdiction in which hakaishield is incorporated, without regard to its conflict of law provisions.
          </p>

          <h2 className="text-2xl font-bold mt-12 mb-4" style={{ color: 'var(--text-primary)' }}>12. Contact Information</h2>
          <p>
            If you have any questions about these Terms, please contact us at:
          </p>
          <p>
            Email: <a href="mailto:legal@hakaishield.io" style={{ color: 'var(--accent-blue)' }}>legal@hakaishield.io</a>
          </p>
        </div>
      </div>
    </div>
  );
}
