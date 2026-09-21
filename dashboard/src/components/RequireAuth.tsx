import { Navigate, Outlet } from 'react-router-dom';
import { isSignedIn } from '../lib/api';

// Gates the dashboard routes behind a signed-in session. A missing or
// expired token (api.ts clears it on any 401) redirects to sign-in
// instead of rendering a dashboard full of failed requests.
export default function RequireAuth() {
  if (!isSignedIn()) {
    return <Navigate to="/sign-in" replace />;
  }
  return <Outlet />;
}
