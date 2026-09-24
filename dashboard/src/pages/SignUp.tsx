export default function SignUp() {
  return (
    <div className="min-h-screen px-6 py-16" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      <div className="max-w-xl mx-auto space-y-6">
        <a href="/landing" className="font-bold text-sm">hakaishield</a>
        <h1 className="text-3xl font-semibold">Pilot access is by invitation</h1>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          The operator creates the first client account after confirming the pilot scope and setup.
          If you already have an account, sign in. To join the pilot, contact the operator.
        </p>
        <div className="flex flex-wrap gap-3">
          <a href="/sign-in" className="btn-primary px-5 py-2.5 text-sm">Sign in</a>
          <a href="/contact" className="btn-secondary px-5 py-2.5 text-sm">Request access</a>
        </div>
      </div>
    </div>
  );
}
