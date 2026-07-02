// In-app feedback API client. Mirrors the src/api/ai.ts pattern but throws a
// status-carrying error so the FeedbackButton can branch on 503 (unconfigured)
// and 429 (rate limited). The backend derives user identity + User-Agent
// server-side; the client only sends category, message, and the current page.
import { getAuthToken, BASE_URL } from './client';

export type FeedbackCategory = 'bug' | 'idea' | 'other';

export interface FeedbackInput {
  category: FeedbackCategory;
  message: string;
  page_url?: string;
}

export interface FeedbackResult {
  issue_url: string;
  issue_number: number;
}

/** Error carrying the HTTP status so callers can branch on 503/429. */
export class FeedbackError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = 'FeedbackError';
    this.status = status;
  }
}

export async function submitFeedback(input: FeedbackInput): Promise<FeedbackResult> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = getAuthToken();
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const response = await fetch(`${BASE_URL}/feedback`, {
    method: 'POST',
    headers,
    body: JSON.stringify(input),
  });

  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    const msg =
      typeof data?.error === 'object' && data.error !== null ? data.error.message : data?.error;
    throw new FeedbackError(response.status, msg || `Request failed with status ${response.status}`);
  }
  return data as FeedbackResult;
}
