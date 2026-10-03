import { expect, it } from 'vitest';
import { consentLoginPath, consentRequestID, loginConsentReturn, validConsentReturn } from './mcpReturnContext';
const id = '55555555-1111-2222-3333-444444444444';
const path = `/mcp/consent?request_id=${id}`;
it('accepts only a single exact consent request destination', () => {
  expect(validConsentReturn(path)).toBe(path);
  expect(consentLoginPath(id)).toBe(`/login?next=${encodeURIComponent(path)}`);
  expect(loginConsentReturn(`?next=${encodeURIComponent(path)}`)).toBe(path);
  expect(consentRequestID(`?request_id=${id}`)).toBe(id);
  for (const value of ['https://evil.example', '//evil.example', '/account', `${path}&other=1`, '/mcp/consent?request_id=invalid', '/mcp/consent?request_id=00000000-0000-0000-0000-000000000000']) expect(validConsentReturn(value)).toBeUndefined();
  expect(consentRequestID(`?request_id=${id}&request_id=${id}`)).toBeUndefined();
  expect(loginConsentReturn(`?next=${encodeURIComponent(path)}&next=${encodeURIComponent(path)}`)).toBeUndefined();
});
