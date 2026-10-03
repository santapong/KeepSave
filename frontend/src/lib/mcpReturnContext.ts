const uuid = '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}';
const consentPath = new RegExp(`^/mcp/consent\\?request_id=(${uuid})$`, 'i');
export function validConsentReturn(value: unknown): string | undefined {
  if (typeof value !== 'string') return undefined;
  const match = consentPath.exec(value);
  return match && match[1] !== '00000000-0000-0000-0000-000000000000' ? `/mcp/consent?request_id=${match[1].toLowerCase()}` : undefined;
}
export function consentRequestID(search: string): string | undefined {
  const params = new URLSearchParams(search);
  if (params.getAll('request_id').length !== 1 || [...params.keys()].some((key) => key !== 'request_id')) return undefined;
  return validConsentReturn(`/mcp/consent?request_id=${params.get('request_id')}`)?.split('=')[1];
}
export function loginConsentReturn(search: string): string | undefined {
  const params = new URLSearchParams(search);
  return params.getAll('next').length === 1 ? validConsentReturn(params.get('next')) : undefined;
}
export function consentLoginPath(requestID: string): string {
  const destination = validConsentReturn(`/mcp/consent?request_id=${requestID}`);
  return destination ? `/login?next=${encodeURIComponent(destination)}` : '/login';
}
