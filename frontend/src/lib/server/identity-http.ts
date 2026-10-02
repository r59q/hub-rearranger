import { resolve } from '$app/paths';
import type { RequestEvent } from '@sveltejs/kit';
import { IdentityApiError, identityClient } from './identity-api';

const names = ['hub_session', '__Host-hub_session', 'hub_login', '__Host-hub_login'];

export function identityFor(event: Pick<RequestEvent, 'request' | 'fetch' | 'url'>) {
	// Keep duplicate cookies so the Go boundary can reject ambiguous identities.
	const cookie = (event.request.headers.get('cookie') ?? '')
		.split(';')
		.map((part) => part.trim())
		.filter((part) => names.includes(part.split('=')[0]))
		.join('; ');

	return identityClient(event.fetch, cookie, event.url.origin);
}

export function publicRedirect(location: string, cookies: string[] = []): Response {
	const headers = new Headers({
		location: location,
		'cache-control': 'no-store',
		'referrer-policy': 'no-referrer'
	});

	for (const cookie of cookies) {
		headers.append('set-cookie', cookie);
	}

	return new Response(null, { status: 303, headers });
}

export function identityFailure(error: unknown): Response {
	const notice =
		error instanceof IdentityApiError && error.status === 401
			? 'reconnect'
			: error instanceof IdentityApiError && error.status === 403
				? 'request-rejected'
				: 'unavailable';

	return publicRedirect(resolve('/account') + '?' + new URLSearchParams({ notice }));
}

export function sameOrigin(event: Pick<RequestEvent, 'request' | 'url'>): boolean {
	return event.request.headers.get('origin') === event.url.origin;
}
