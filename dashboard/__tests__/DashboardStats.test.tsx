import { render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import DashboardStats from '../src/components/DashboardStats';

const mockStats = {
  total_requests: 15000,
  passed: 12500,
  challenged: 2000,
  blocked: 500,
  mode: 'enforce' as const,
  enforcing: true,
};

const mockShadowStats = { ...mockStats, mode: 'shadow' as const, enforcing: false };

function mockFetchOnce(status: number, body: unknown) {
  global.fetch = jest.fn().mockResolvedValueOnce({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  }) as jest.Mock;
}

describe('DashboardStats', () => {
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('renders loading state initially', () => {
    mockFetchOnce(200, mockStats);
    render(<DashboardStats />);
    expect(screen.getByText('Analyzing Network Traffic...')).toBeInTheDocument();
  });

  it('renders real stats fetched from the backend', async () => {
    mockFetchOnce(200, mockStats);
    render(<DashboardStats />);

    await waitFor(() => {
      expect(screen.getByText('Total Requests')).toBeInTheDocument();
    });

    expect(screen.getByText(mockStats.total_requests.toLocaleString())).toBeInTheDocument();
    expect(screen.getByText(mockStats.passed.toLocaleString())).toBeInTheDocument();
    expect(screen.getByText(mockStats.challenged.toLocaleString())).toBeInTheDocument();
    expect(screen.getByText(mockStats.blocked.toLocaleString())).toBeInTheDocument();
    expect(global.fetch).toHaveBeenCalledWith('http://localhost:8080/api/v1/dashboard/stats');
  });

  // CLAUDE.md Section 23b: "only the easy input doesn't prove
  // robustness" - a real backend being down/unreachable is the normal
  // case for a bot-detection dashboard, not an edge case.
  it('renders error state when the backend request fails', async () => {
    global.fetch = jest.fn().mockRejectedValueOnce(new Error('network down'));
    render(<DashboardStats />);

    await waitFor(() => {
      expect(screen.getByText('Connection Lost')).toBeInTheDocument();
    });
    expect(
      screen.getByText('Failed to load dashboard statistics. Please try again.')
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry Connection' })).toBeInTheDocument();
    expect(screen.queryByText('Total Requests')).not.toBeInTheDocument();
  });

  // A non-2xx response (backend up, but erroring) must be treated as
  // a failure too, not rendered as if it were real data.
  it('renders error state when the backend returns a non-OK status', async () => {
    mockFetchOnce(500, { error: 'internal error' });
    render(<DashboardStats />);

    await waitFor(() => {
      expect(screen.getByText('Connection Lost')).toBeInTheDocument();
    });
  });
});

// Shadow mode's one job is to be impossible to mistake for
// enforcement. A customer who thinks they are protected while nothing
// is being blocked is worse off than one with no bot-shield at all
// (docs/ROADMAP.md item 18).
describe('DashboardStats in shadow mode', () => {
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('warns that nothing is being blocked', async () => {
    mockFetchOnce(200, mockShadowStats);
    render(<DashboardStats />);

    await waitFor(() => {
      expect(
        screen.getByText('Shadow mode — nothing is being blocked.')
      ).toBeInTheDocument();
    });
  });

  it('labels the counts as hypothetical, never as things that happened', async () => {
    mockFetchOnce(200, mockShadowStats);
    render(<DashboardStats />);

    await waitFor(() => {
      expect(screen.getByText('Would block')).toBeInTheDocument();
    });
    expect(screen.getByText('Would challenge')).toBeInTheDocument();
    expect(screen.getByText('Would pass')).toBeInTheDocument();

    // The enforcing labels must be gone entirely - not merely
    // accompanied by a warning somewhere else on the page.
    expect(screen.queryByText('Blocked')).not.toBeInTheDocument();
    expect(screen.queryByText('Challenged')).not.toBeInTheDocument();
    expect(screen.queryByText('Passed')).not.toBeInTheDocument();
  });

  it('shows no shadow warning and real labels when enforcing', async () => {
    mockFetchOnce(200, mockStats);
    render(<DashboardStats />);

    await waitFor(() => {
      expect(screen.getByText('Blocked')).toBeInTheDocument();
    });
    expect(
      screen.queryByText('Shadow mode — nothing is being blocked.')
    ).not.toBeInTheDocument();
    expect(screen.queryByText('Would block')).not.toBeInTheDocument();
  });
});

// The old header badge read "System Active" unconditionally, so it
// claimed the product was working even with the backend down or
// shadow mode on. Its replacement may only say what is true.
describe('status badge honesty', () => {
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('says enforcing only when actually enforcing', async () => {
    mockFetchOnce(200, mockStats);
    render(<DashboardStats />);

    await waitFor(() => {
      expect(screen.getByText('Enforcing')).toBeInTheDocument();
    });
    expect(screen.queryByText('Shadow mode — not enforcing')).not.toBeInTheDocument();
  });

  it('says not enforcing in shadow mode', async () => {
    mockFetchOnce(200, mockShadowStats);
    render(<DashboardStats />);

    await waitFor(() => {
      expect(screen.getByText('Shadow mode — not enforcing')).toBeInTheDocument();
    });
    expect(screen.queryByText('Enforcing')).not.toBeInTheDocument();
  });

  it('claims nothing at all when the backend is unreachable', async () => {
    global.fetch = jest.fn().mockRejectedValueOnce(new Error('network down'));
    render(<DashboardStats />);

    await waitFor(() => {
      expect(screen.getByText('Connection Lost')).toBeInTheDocument();
    });
    expect(screen.queryByText('Enforcing')).not.toBeInTheDocument();
    expect(screen.queryByText('Shadow mode — not enforcing')).not.toBeInTheDocument();
  });
});

// A response the dashboard can't fully understand must produce the
// error state, never a guess. `enforcing` missing would read as false
// and announce "nothing is being blocked" while the proxy enforces —
// the shadow-mode lie, inverted.
describe('malformed stats responses', () => {
  afterEach(() => {
    jest.restoreAllMocks();
  });

  const badBodies: Array<[string, unknown]> = [
    ['missing enforcing', { total_requests: 1, passed: 1, challenged: 0, blocked: 0, mode: 'enforce' }],
    ['missing mode', { total_requests: 1, passed: 1, challenged: 0, blocked: 0, enforcing: true }],
    ['unknown mode', { total_requests: 1, passed: 1, challenged: 0, blocked: 0, mode: 'observe', enforcing: true }],
    ['counter is a string', { total_requests: '1', passed: 1, challenged: 0, blocked: 0, mode: 'enforce', enforcing: true }],
    ['counter missing', { passed: 1, challenged: 0, blocked: 0, mode: 'enforce', enforcing: true }],
    ['empty body', {}],
    ['null body', null],
  ];

  it.each(badBodies)('shows the error state and claims nothing: %s', async (_name, body) => {
    mockFetchOnce(200, body);
    render(<DashboardStats />);

    await waitFor(() => {
      expect(screen.getByText('Connection Lost')).toBeInTheDocument();
    });
    expect(
      screen.queryByText('Shadow mode — nothing is being blocked.')
    ).not.toBeInTheDocument();
    expect(screen.queryByText('Enforcing')).not.toBeInTheDocument();
    expect(screen.queryByText('Shadow mode — not enforcing')).not.toBeInTheDocument();
    expect(screen.queryByText('Total Requests')).not.toBeInTheDocument();
  });
});
