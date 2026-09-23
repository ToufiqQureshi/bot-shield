import { lazy, Suspense } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { ThemeProvider } from './context/ThemeContext';
import Layout from './components/Layout';
import RequireAuth from './components/RequireAuth';

const Overview = lazy(() => import('./pages/Overview'));
const EvidenceLogs = lazy(() => import('./pages/EvidenceLogs'));
const MitigationRules = lazy(() => import('./pages/MitigationRules'));
const ProtectionSettings = lazy(() => import('./pages/ProtectionSettings'));
const DomainsSiem = lazy(() => import('./pages/DomainsSiem'));
const Landing = lazy(() => import('./pages/Landing'));
const Pricing = lazy(() => import('./pages/Pricing'));
const Changelog = lazy(() => import('./pages/Changelog'));
const Docs = lazy(() => import('./pages/Docs'));
const Contact = lazy(() => import('./pages/Contact'));
const SignIn = lazy(() => import('./pages/SignIn'));
const SignUp = lazy(() => import('./pages/SignUp'));
const ForgotPassword = lazy(() => import('./pages/ForgotPassword'));
const Onboarding = lazy(() => import('./pages/Onboarding'));
const Subscription = lazy(() => import('./pages/Subscription'));
const Payment = lazy(() => import('./pages/Payment'));
const About = lazy(() => import('./pages/About'));
const Terms = lazy(() => import('./pages/Terms'));
const Privacy = lazy(() => import('./pages/Privacy'));

function PageLoader() {
  return (
    <div className="flex items-center justify-center min-h-[50vh]">
      <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin"></div>
    </div>
  );
}

function App() {
  return (
    <ThemeProvider>
      <BrowserRouter>
        <Suspense fallback={<PageLoader />}>
          <Routes>
            {/* Marketing Pages (no dashboard layout) */}
            <Route path="/landing" element={<Landing />} />
            <Route path="/pricing" element={<Pricing />} />
            <Route path="/changelog" element={<Changelog />} />
            <Route path="/docs" element={<Docs />} />
            <Route path="/contact" element={<Contact />} />
            <Route path="/about" element={<About />} />
            <Route path="/terms" element={<Terms />} />
            <Route path="/privacy" element={<Privacy />} />

            {/* Authentication Pages */}
            <Route path="/sign-in" element={<SignIn />} />
            <Route path="/sign-up" element={<SignUp />} />
            <Route path="/forgot-password" element={<ForgotPassword />} />
            <Route path="/onboarding" element={<Onboarding />} />

            {/* Subscription & Payment */}
            <Route path="/subscription" element={<Subscription />} />
            <Route path="/payment" element={<Payment />} />

            {/* Dashboard (requires a signed-in session) */}
            <Route element={<RequireAuth />}>
              <Route path="/" element={<Layout />}>
                <Route index element={<Overview />} />
                <Route path="evidence-logs" element={<EvidenceLogs />} />
                <Route path="mitigation-rules" element={<MitigationRules />} />
                <Route path="protection-settings" element={<ProtectionSettings />} />
                <Route path="domains-siem" element={<DomainsSiem />} />
              </Route>
            </Route>
          </Routes>
        </Suspense>
      </BrowserRouter>
    </ThemeProvider>
  );
}

export default App;
