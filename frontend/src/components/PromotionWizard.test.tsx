import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PromotionWizard } from './PromotionWizard';

vi.mock('../api/client', () => ({
  promoteDiff: vi.fn(),
  promote: vi.fn(),
}));

import { promoteDiff, promote } from '../api/client';

const mockDiff = [
  { key: 'DB_HOST', action: 'add' as const, source_exists: true, target_exists: false },
  { key: 'API_URL', action: 'update' as const, source_exists: true, target_exists: true },
  { key: 'SHARED', action: 'no_change' as const, source_exists: true, target_exists: true },
];

describe('PromotionWizard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (promoteDiff as ReturnType<typeof vi.fn>).mockResolvedValue(mockDiff);
    (promote as ReturnType<typeof vi.fn>).mockResolvedValue({
      id: 'promo-1',
      status: 'completed',
    });
  });

  it('renders configure step with promotion paths', () => {
    render(<PromotionWizard projectId="p1" />);
    expect(screen.getByText('Configure Promotion')).toBeInTheDocument();
    expect(screen.getByText('Alpha → UAT')).toBeInTheDocument();
    expect(screen.getByText('UAT → PROD')).toBeInTheDocument();
  });

  it('loads diff preview and shows review step', async () => {
    const user = userEvent.setup();
    render(<PromotionWizard projectId="p1" />);

    await user.click(screen.getByText('Preview Changes'));

    await waitFor(() => {
      expect(screen.getByText('DB_HOST')).toBeInTheDocument();
      expect(screen.getByText('API_URL')).toBeInTheDocument();
      expect(screen.getByText('SHARED')).toBeInTheDocument();
    });

    expect(promoteDiff).toHaveBeenCalledWith('p1', 'alpha', 'uat');
  });

  it('executes promotion and shows success', async () => {
    const user = userEvent.setup();
    render(<PromotionWizard projectId="p1" />);

    await user.click(screen.getByText('Preview Changes'));
    await waitFor(() => expect(screen.getByText('DB_HOST')).toBeInTheDocument());

    await user.click(screen.getByText('Promote to UAT'));
    await waitFor(() => {
      expect(screen.getByText('Promotion completed successfully!')).toBeInTheDocument();
    });

    expect(promote).toHaveBeenCalled();
  });

  it('shows pending message for PROD promotions', async () => {
    (promote as ReturnType<typeof vi.fn>).mockResolvedValue({
      id: 'promo-1',
      status: 'pending',
    });

    const user = userEvent.setup();
    render(<PromotionWizard projectId="p1" />);

    await user.click(screen.getByText('UAT → PROD'));
    await user.click(screen.getByText('Preview Changes'));
    await waitFor(() => expect(screen.getByText('DB_HOST')).toBeInTheDocument());

    await user.click(screen.getByText('Request PROD Promotion'));
    await waitFor(() => {
      expect(screen.getByText(/require approval/)).toBeInTheDocument();
    });
  });

  it('navigates back from review to configure', async () => {
    const user = userEvent.setup();
    render(<PromotionWizard projectId="p1" />);

    await user.click(screen.getByText('Preview Changes'));
    await waitFor(() => expect(screen.getByText('DB_HOST')).toBeInTheDocument());

    await user.click(screen.getByText('Back'));
    expect(screen.getByText('Configure Promotion')).toBeInTheDocument();
  });
});
