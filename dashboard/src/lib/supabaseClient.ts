import { createClient } from '@supabase/supabase-js';

// Supabase handles account creation, sign-in, email verification, and
// password reset directly — this dashboard no longer implements any
// of that itself (see docs/DECISIONS.md's Supabase-migration entry).
// The Go backend (src/lib/api.ts) only verifies the session token
// Supabase already issued; it never sees a password.
const url = import.meta.env.VITE_SUPABASE_URL;
const anonKey = import.meta.env.VITE_SUPABASE_ANON_KEY;

if (!url || !anonKey) {
  // Fails loudly at startup instead of every auth call silently
  // producing a confusing "invalid URL" error deep in supabase-js.
  throw new Error(
    'VITE_SUPABASE_URL and VITE_SUPABASE_ANON_KEY must be set (see dashboard/.env.example).'
  );
}

export const supabase = createClient(url, anonKey);
