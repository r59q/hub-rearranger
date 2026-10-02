import { describe, expect, it, vi } from 'vitest';
import type { RequestEvent } from '@sveltejs/kit';
import { POST as start } from './start/+server';
import { GET as callback } from './callback/+server';
import { POST as signOut } from './sign-out/+server';
import { identityFor } from '$lib/server/identity-http';

function event(path: string, method: string, request: typeof fetch, form?: URLSearchParams) {
	const url = new URL(path, 'https://hub.example.com');
	return {
		url,
		fetch: request,
		request: new Request(url, {
			method,
			headers: {
				origin: url.origin,
				cookie: 'hub_session=' + 's'.repeat(43) + '; unrelated=private-sentinel'
			},
			body: form
		})
	} as unknown as RequestEvent;
}
describe('identity browser routes', () => {
	it('requires the browser origin before creating a login intent', async () => {
		const request = vi.fn<typeof fetch>();
		const input = event('/auth/start', 'POST', request);
		input.request.headers.set('origin', 'https://attacker.example');

		const response = await start(input as never);

		expect(response.status).toBe(403);
		expect(request).not.toHaveBeenCalled();
	});

	it('forwards opaque cookies and the reviewed GitHub redirect', async () => {
		const request = vi.fn<typeof fetch>().mockResolvedValue(
			new Response(null, {
				status: 303,
				headers: {
					location: 'https://github.com/login/oauth/authorize?state=synthetic',
					'set-cookie':
						'__Host-hub_login=' + 'b'.repeat(43) + '; Path=/; HttpOnly; Secure; SameSite=Lax'
				}
			})
		);

		const response = await start(event('/auth/start', 'POST', request) as never);

		expect(response.status).toBe(303);
		expect(response.headers.get('set-cookie')).toContain('HttpOnly');
		expect(response.headers.get('cache-control')).toBe('no-store');
		expect(new Headers(request.mock.calls[0][1]?.headers).get('cookie')).not.toContain(
			'private-sentinel'
		);
	});

	it.each([
		'/auth/callback?code=one&code=two&state=x',
		'/auth/callback?error=access_denied',
		'/auth/callback?code=x'
	])('rejects ambiguous or declined callback %s', async (path) => {
		const request = vi.fn<typeof fetch>();

		const response = await callback(event(path, 'GET', request) as never);

		expect(response.headers.get('location')).toContain('request-rejected');
		expect(request).not.toHaveBeenCalled();
	});

	it('preserves the service-created session without putting it in page data', async () => {
		const request = vi.fn<typeof fetch>().mockResolvedValue(
			new Response(null, {
				status: 303,
				headers: {
					location: 'https://hub.example.com/account',
					'set-cookie':
						'__Host-hub_session=' + 's'.repeat(43) + '; Path=/; HttpOnly; Secure; SameSite=Lax'
				}
			})
		);

		const response = await callback(
			event('/auth/callback?code=synthetic&state=synthetic', 'GET', request) as never
		);

		expect(response.headers.get('location')).toBe('https://hub.example.com/account');
		expect(await response.text()).toBe('');
	});

	it('rejects sign-out without a session nonce before requesting the service', async () => {
		const request = vi.fn<typeof fetch>();

		const response = await signOut(
			event('/auth/sign-out', 'POST', request, new URLSearchParams()) as never
		);

		expect(response.headers.get('location')).toContain('request-rejected');
		expect(request).not.toHaveBeenCalled();
	});

	it('keeps failed revocation visible after local sign-out', async () => {
		const request = vi
			.fn<typeof fetch>()
			.mockResolvedValue(
				Response.json(
					{ revoked: false, message: 'private-sentinel' },
					{ headers: { 'set-cookie': '__Host-hub_session=; Path=/; Max-Age=0; HttpOnly; Secure' } }
				)
			);

		const response = await signOut(
			event(
				'/auth/sign-out',
				'POST',
				request,
				new URLSearchParams({ csrf: 'c'.repeat(43) })
			) as never
		);

		expect(response.headers.get('location')).toContain('revocation-unconfirmed');
		expect(response.headers.get('set-cookie')).toContain('Max-Age=0');
	});

	it('filters unrelated cookies and preserves duplicate identity cookies for rejection', async () => {
		const request = vi
			.fn<typeof fetch>()
			.mockResolvedValue(
				Response.json({ state: 'signed_out', user: null, csrf: null, expires_at: null })
			);
		const input = event('/account', 'GET', request);
		input.request.headers.set(
			'cookie',
			'unrelated=private-sentinel; hub_session=' +
				's'.repeat(43) +
				'; hub_session=' +
				't'.repeat(43)
		);
		await identityFor(input).session();
		const outgoing = request.mock.calls[0][0] as Request;
		expect(outgoing.headers.get('cookie')).not.toContain('private-sentinel');
		expect(outgoing.headers.get('cookie')).toContain('; hub_session=');
	});
});
