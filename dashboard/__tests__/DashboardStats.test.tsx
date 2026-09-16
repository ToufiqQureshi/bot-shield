import { render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import DashboardStats from '../src/components/DashboardStats';

const mockStats = {
  total_requests: 15000,
  passed: 12500,
  challenged: 2000,
  blocked: 500,
};

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
