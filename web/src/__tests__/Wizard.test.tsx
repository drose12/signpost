import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { Wizard } from '../pages/Wizard';
import { setCredentials, clearCredentials } from '../api';

describe('Wizard', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    clearCredentials();
    setCredentials('test', 'test');
    // Mock fetch to return empty domains (first-run)
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve([]),
    });
  });

  it('renders all 5 steps', async () => {
    render(
      <BrowserRouter>
        <Wizard />
      </BrowserRouter>
    );

    // Wait for domains fetch to complete and wizard to render.
    // "Add Domain" appears multiple times (sidebar, heading, button), so wait
    // for any instance, then verify all step titles are present in the sidebar.
    await screen.findAllByText('Add Domain', {}, { timeout: 3000 });

    // Each step title appears in the sidebar as a <span> inside a <button>.
    // Use getAllByText to handle duplicates and verify at least one match each.
    expect(screen.getAllByText('Add Domain').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('Generate DKIM')).toBeInTheDocument();
    expect(screen.getByText('DNS Records')).toBeInTheDocument();
    expect(screen.getByText('Configure Relay')).toBeInTheDocument();
    expect(screen.getByText('Send Test Email')).toBeInTheDocument();
  });
});
