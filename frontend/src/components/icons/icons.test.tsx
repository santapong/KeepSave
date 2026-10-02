import { createRef } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { FolderClosed, Search } from './index';

describe('KeepSave icon accessibility', () => {
  it('keeps a control name and interaction independent of its decorative icon', async () => {
    const onClick = vi.fn();
    render(<button onClick={onClick}><Search size={16} />Search projects</button>);
    const button = screen.getByRole('button', { name: 'Search projects' });
    expect(button.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
    await userEvent.click(button);
    expect(onClick).toHaveBeenCalledOnce();
  });

  it('supports named standalone icons and shared SVG styling', () => {
    const ref = createRef<SVGSVGElement>();
    render(<FolderClosed ref={ref} title="Project vault" size={32} className="text-primary" strokeWidth={2} />);
    const icon = screen.getByRole('img', { name: 'Project vault' });
    expect(icon).not.toHaveAttribute('aria-hidden');
    expect(icon).toHaveAttribute('width', '32');
    expect(icon).toHaveAttribute('stroke-width', '2');
    expect(icon).toHaveClass('text-primary');
    expect(ref.current).toBe(icon);
  });
});
