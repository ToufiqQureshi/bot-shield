import type { PolicyDocument } from './api';

// The backend is authoritative on validation and activation. This guards
// accidental UI edits and preserves unrelated policy settings in the draft.
export function routeLabelDraft(current: PolicyDocument | null, rawPath: string, className: 'login' | 'checkout' | null): PolicyDocument {
  if (current?.mode === 'enforce') {
    throw new Error('Active policy changes need operator review.');
  }
  const path = rawPath.trim();
  if (!path.startsWith('/') || path === '/' || path.length > 256 || /[?#%\\\s]/.test(path) || path.includes('//') || path.split('/').some((part) => part === '.' || part === '..')) {
    throw new Error('Enter one exact path, such as /account/signin.');
  }
  const routeClasses = { ...current?.routeClasses };
  if (className === null) {
    if (!(path in routeClasses)) throw new Error('Route tag no longer exists. Refresh and try again.');
    delete routeClasses[path];
  } else {
    if (path in routeClasses) throw new Error('This route is already tagged.');
    routeClasses[path] = className;
  }
  return { ...(current ?? { mode: 'shadow', rules: [] }), mode: 'shadow', routeClasses };
}
