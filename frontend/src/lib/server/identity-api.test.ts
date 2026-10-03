import { describe, expect, it, vi } from 'vitest';
import { IdentityApiClient, IdentityApiError } from './identity-api';
import { base, digest, publication } from '../../test/bootstrap-fixtures';

const session = {
	state: 'authenticated',
	user: { id: 7, login: 'octocat' },
	csrf: 'c'.repeat(43),
	expires_at: '2026-10-08T12:00:00Z'
};

const client = (request: typeof fetch) =>
	new IdentityApiClient(
		'http://identity:8083',
		request,
		'hub_session=' + 's'.repeat(43),
		'https://hub.example.com'
	);
describe('identity server adapter', () => {
	it('maps safe identity fields and keeps the opaque cookie on server requests', async () => {
		const request = vi.fn<typeof fetch>().mockResolvedValue(Response.json(session));

		const result = await client(request).session();

		expect(result).toEqual({
			state: 'authenticated',
			user: session.user,
			csrf: session.csrf,
			expiresAt: session.expires_at
		});
		const outgoing = request.mock.calls[0][0] as Request;
		expect(outgoing.headers.get('cookie')).toBe('hub_session=' + 's'.repeat(43));
		expect(outgoing.credentials).toBe('omit');
		expect(outgoing.headers.has('authorization')).toBe(false);
		expect(JSON.stringify(result)).not.toContain('hub_session');
	});

	it.each(['disabled', 'signed_out'])(
		'maps %s without claiming a connected user',
		async (state) => {
			const request = vi
				.fn<typeof fetch>()
				.mockResolvedValue(Response.json({ state, user: null, csrf: null, expires_at: null }));
			expect((await client(request).session()).state).toBe(state);
		}
	);

	it.each([401, 403, 503])('handles %i without relaying provider prose', async (status) => {
		const request = vi
			.fn<typeof fetch>()
			.mockResolvedValue(Response.json({ message: 'private-sentinel' }, { status }));

		const result = await client(request).session();

		expect(result.state).toBe(status === 401 ? 'reconnect_required' : 'unavailable');
		expect(JSON.stringify(result)).not.toContain('private-sentinel');
	});

	it.each([
		{ ...session, access_token: 'synthetic-never-live' },
		{ ...session, csrf: 'bad' },
		{ ...session, user: { id: 7, login: '<script>bad</script>' } },
		{ ...session, expires_at: 'bad' }
	])('rejects malformed or unexpected credential fields', async (value) => {
		expect(
			(await client(vi.fn<typeof fetch>().mockResolvedValue(Response.json(value))).session()).state
		).toBe('unavailable');
	});

	it('never follows OAuth redirects on the server', async () => {
		const response = new Response(null, {
			status: 303,
			headers: { location: 'https://github.com/login/oauth/authorize?state=synthetic-state' }
		});
		const request = vi.fn<typeof fetch>().mockResolvedValue(response);
		expect(await client(request).start()).toBe(response);

		expect(request.mock.calls[0][1]?.redirect).toBe('manual');
		expect(request.mock.calls[0][1]?.method).toBe('POST');
	});

	it('rejects untrusted redirect locations', async () => {
		const request = vi
			.fn<typeof fetch>()
			.mockResolvedValue(
				new Response(null, { status: 303, headers: { location: 'https://attacker.example' } })
			);

		await expect(client(request).start()).rejects.toBeInstanceOf(IdentityApiError);
	});

	it('encodes the callback and allows only the fixed account destination', async () => {
		const request = vi.fn<typeof fetch>().mockResolvedValue(
			new Response(null, {
				status: 303,
				headers: { location: 'https://hub.example.com/account' }
			})
		);

		await client(request).complete('code&extra=bad', 'state/value');

		const url = new URL(String(request.mock.calls[0][0]));
		expect(url.searchParams.get('code')).toBe('code&extra=bad');
		expect(url.searchParams.get('state')).toBe('state/value');
	});

	it('sends a protected sign-out and uses fixed recovery text', async () => {
		const request = vi
			.fn<typeof fetch>()
			.mockResolvedValue(Response.json({ revoked: false, message: 'private-sentinel' }));

		const result = await client(request).signOut(session.csrf);

		const outgoing = request.mock.calls[0][0] as Request;
		expect(outgoing.headers.get('origin')).toBe('https://hub.example.com');
		expect(await outgoing.json()).toEqual({ csrf: session.csrf });
		expect(result.message).toContain('GitHub account settings');
		expect(result.message).not.toContain('private-sentinel');
	});
});

describe('bootstrap write adapter', () => {
	it('uses authenticated Origin/session and validates the reviewed result', async () => {
		const request = vi.fn<typeof fetch>().mockResolvedValue(Response.json(publication));
		expect(
			await client(request).createBootstrap('octo', 'demo', session.csrf, base, digest)
		).toEqual(publication);
		const outgoing = request.mock.calls[0][0] as Request;
		expect(outgoing.headers.get('origin')).toBe('https://hub.example.com');
		expect(await outgoing.json()).toEqual({ csrf: session.csrf, base_revision: base, digest });
	});
	it.each([
		{ ...publication, pull_request_url: 'https://attacker.example' },
		{ ...publication, branch: 'main' },
		{ ...publication, repository: 'other/repo' },
		{ ...publication, access_token: 'synthetic-never-live' }
	])('rejects forged result fields', async (data) => {
		await expect(
			client(vi.fn<typeof fetch>().mockResolvedValue(Response.json(data))).createBootstrap(
				'octo',
				'demo',
				session.csrf,
				base,
				digest
			)
		).rejects.toBeInstanceOf(IdentityApiError);
	});
});
