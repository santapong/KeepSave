import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { FeedbackButton } from './FeedbackButton';
import { FeedbackError, submitFeedback } from '../api/feedback';

const { toastMock } = vi.hoisted(() => ({ toastMock: vi.fn() }));

vi.mock('@/hooks/useToast', () => ({
  useToast: () => ({ toast: toastMock }),
}));

// Keep the real FeedbackError class (so `instanceof` in the component works)
// but stub the network call.
vi.mock('../api/feedback', async () => {
  const actual = await vi.importActual<typeof import('../api/feedback')>('../api/feedback');
  return { ...actual, submitFeedback: vi.fn() };
});

const submitMock = submitFeedback as ReturnType<typeof vi.fn>;

describe('FeedbackButton', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the floating launcher', () => {
    render(<FeedbackButton />);
    expect(screen.getByRole('button', { name: 'Give feedback' })).toBeInTheDocument();
  });

  it('opens the dialog on launcher click', async () => {
    const user = userEvent.setup();
    render(<FeedbackButton />);
    await user.click(screen.getByRole('button', { name: 'Give feedback' }));
    expect(await screen.findByRole('heading', { name: 'Send feedback' })).toBeInTheDocument();
    expect(screen.getByLabelText('Feedback message')).toBeInTheDocument();
  });

  it('disables submit when the message is empty', async () => {
    const user = userEvent.setup();
    render(<FeedbackButton />);
    await user.click(screen.getByRole('button', { name: 'Give feedback' }));
    expect(screen.getByRole('button', { name: 'Send feedback' })).toBeDisabled();
  });

  it('submits feedback and shows a success toast', async () => {
    submitMock.mockResolvedValue({ issue_url: 'https://x/1', issue_number: 1 });
    const user = userEvent.setup();
    render(<FeedbackButton />);

    await user.click(screen.getByRole('button', { name: 'Give feedback' }));
    await user.type(screen.getByLabelText('Feedback message'), 'the button is broken');
    await user.click(screen.getByRole('button', { name: 'Send feedback' }));

    await waitFor(() => expect(submitMock).toHaveBeenCalledTimes(1));
    expect(submitMock).toHaveBeenCalledWith(
      expect.objectContaining({ category: 'bug', message: 'the button is broken' })
    );
    await waitFor(() =>
      expect(toastMock).toHaveBeenCalledWith(expect.objectContaining({ variant: 'success' }))
    );
  });

  it('shows the friendly message when the server is unconfigured (503)', async () => {
    submitMock.mockRejectedValue(new FeedbackError(503, 'feedback is not configured on this server'));
    const user = userEvent.setup();
    render(<FeedbackButton />);

    await user.click(screen.getByRole('button', { name: 'Give feedback' }));
    await user.type(screen.getByLabelText('Feedback message'), 'hello');
    await user.click(screen.getByRole('button', { name: 'Send feedback' }));

    await waitFor(() =>
      expect(toastMock).toHaveBeenCalledWith(
        expect.objectContaining({
          variant: 'destructive',
          title: "Feedback isn't configured on this server",
        })
      )
    );
  });
});
