import assert from 'node:assert/strict';
import test from 'node:test';
import { domainState } from '../src/lib/domainStatus.ts';

test('only an active domain is presented as protected', () => {
  for (const status of ['pending_verification', 'failed', '', 'ACTIVE']) {
    const state = domainState(status);
    assert.equal(state.protected, false, status);
    assert.match(state.detail, /not routing|not confirmed/);
  }
  assert.equal(domainState('active').protected, true);
});
