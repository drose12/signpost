import { describe, it, expect, vi, beforeEach } from 'vitest';
import { setCredentials, clearCredentials, hasCredentials, api } from '../api';

describe('API client', () => {
  beforeEach(() => {
    clearCredentials();
    vi.restoreAllMocks();
  });

  it('tracks credentials', () => {
    expect(hasCredentials()).toBe(false);
    setCredentials('admin', 'pass');
    expect(hasCredentials()).toBe(true);
    clearCredentials();
    expect(hasCredentials()).toBe(false);
  });

  it('sends auth header', async () => {
    setCredentials('admin', 'pass');
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ data: 'test' }),
    });
    global.fetch = mockFetch;

    await api.get('/status');

    expect(mockFetch).toHaveBeenCalledWith('/api/v1/status', expect.objectContaining({
      headers: expect.objectContaining({
        Authorization: 'Basic ' + btoa('admin:pass'),
      }),
    }));
  });

  it('clears credentials on 401', async () => {
    setCredentials('admin', 'wrong');
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ error: 'Unauthorized' }),
    });

    await expect(api.get('/status')).rejects.toThrow('Unauthorized');
    expect(hasCredentials()).toBe(false);
  });

  it('throws ApiError on non-OK response', async () => {
    setCredentials('admin', 'pass');
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ error: 'Internal server error' }),
    });

    await expect(api.get('/status')).rejects.toThrow('Internal server error');
  });
});
