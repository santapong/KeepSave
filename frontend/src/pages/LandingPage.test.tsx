import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { LandingPage } from './LandingPage';

function renderLanding() {
  return render(<MemoryRouter><LandingPage /></MemoryRouter>);
}

describe('LandingPage walkthrough', () => {
  it('preserves independent vault and scoped-access selections between chapters', async () => {
    const user = userEvent.setup();
    renderLanding();
    await user.click(screen.getByRole('button', { name: /PROD Run/ }));
    expect(screen.getByRole('table', { name: 'PROD example secrets' })).toBeVisible();
    await user.click(screen.getByRole('tab', { name: /Scope the access/ }));
    const group = screen.getByRole('group', { name: 'Example agent access environment' });
    expect(within(group).getByRole('button', { name: /UAT Test/ })).toHaveAttribute('aria-pressed', 'true');
    await user.click(within(group).getByRole('button', { name: /Alpha Build/ }));
    expect(screen.getByText('Read secrets in Alpha')).toBeVisible();
    await user.click(screen.getByRole('tab', { name: /Store your secrets/ }));
    expect(screen.getByRole('table', { name: 'PROD example secrets' })).toBeVisible();
    await user.click(screen.getByRole('tab', { name: /Scope the access/ }));
    expect(screen.getByText('Read secrets in Alpha')).toBeVisible();
  });

  it('moves tab selection and focus with arrow, Home and End keys', async () => {
    const user = userEvent.setup();
    renderLanding();
    const store = screen.getByRole('tab', { name: /Store your secrets/ });
    const scope = screen.getByRole('tab', { name: /Scope the access/ });
    const review = screen.getByRole('tab', { name: /Review the change/ });
    await user.click(store);
    await user.keyboard('{ArrowLeft}');
    expect(review).toHaveFocus();
    expect(review).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tabpanel')).toHaveAttribute('aria-labelledby', review.id);
    await user.keyboard('{Home}{ArrowRight}');
    expect(scope).toHaveFocus();
    await user.keyboard('{End}{ArrowRight}');
    expect(store).toHaveFocus();
  });

  it('dismisses mobile navigation with Escape, outside interaction and a selected link', async () => {
    const user = userEvent.setup();
    renderLanding();
    await user.click(screen.getByRole('button', { name: 'Open navigation' }));
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('navigation', { name: 'Mobile navigation' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Open navigation' })).toHaveFocus();
    await user.click(screen.getByRole('button', { name: 'Open navigation' }));
    await user.click(screen.getByRole('heading', { level: 1 }));
    expect(screen.queryByRole('navigation', { name: 'Mobile navigation' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Open navigation' }));
    await user.click(within(screen.getByRole('navigation', { name: 'Mobile navigation' })).getByRole('link', { name: 'Product' }));
    expect(screen.queryByRole('navigation', { name: 'Mobile navigation' })).not.toBeInTheDocument();
  });

  it('keeps the product and primary action available when hero art fails', () => {
    const { container } = renderLanding();
    fireEvent.error(container.querySelector('.ks-hero-art img')!);
    expect(container.querySelector('.ks-art-fallback')).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 1 })).toBeVisible();
    expect(screen.getAllByRole('link', { name: 'Create your vault' })[0]).toHaveAttribute('href', '/register');
  });
});
