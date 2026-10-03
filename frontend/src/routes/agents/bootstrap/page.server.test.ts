import { describe, expect, it, vi } from 'vitest';
import { actions, load } from './+page.server';
import { repository } from '../../../test/agent-fixtures';
import { base, digest, identity, preview, publication } from '../../../test/bootstrap-fixtures';

function requestFor(options: { status?: number; selected?: boolean; identityState?: string } = {}) {
	return vi.fn<typeof fetch>().mockImplementation(async (input) => {
		const request = input instanceof Request ? input : new Request(String(input));
		const path = new URL(request.url).pathname;
		if (path === '/v1/repositories') {
			return Response.json({ repositories: options.selected === false ? [] : [repository] });
		}
		if (path === '/v1/session') {
			return Response.json({
				state: options.identityState ?? 'authenticated',
				user: identity.user,
				csrf: identity.csrf,
				expires_at: identity.expiresAt
			});
		}
		if (path.endsWith('/bootstrap')) {
			return Response.json(preview());
		}
		if (path.endsWith('/bootstrap-pull-request')) {
			return Response.json(
				options.status ? { code: 'bootstrap_stale', message: 'private-sentinel' } : publication,
				{ status: options.status ?? 200 }
			);
		}
		return new Response(null, { status: 404 });
	});
}

function event(
	fetch: typeof globalThis.fetch,
	changes: Record<string, string> = {},
	origin = 'http://localhost',
	query = 'octo/demo'
) {
	return {
		fetch,
		url: new URL('http://localhost/agents/bootstrap?' + new URLSearchParams({ repository: query })),
		setHeaders: vi.fn(),
		request: new Request('http://localhost/agents/bootstrap', {
			method: 'POST',
			headers: { origin, cookie: 'hub_session=' + 's'.repeat(43) },
			body: new URLSearchParams({
				csrf: identity.csrf!,
				base_revision: base,
				digest,
				confirm: 'reviewed',
				...changes
			})
		})
	};
}

describe('bootstrap review server workflow', () => {
	it('loads fresh preview and identity only on the setup page', async () => {
		const input = event(requestFor());
		expect(await load(input as never)).toMatchObject({
			identity,
			repository,
			preview: { state: 'ready' },
			error: null
		});
		expect(input.setHeaders).toHaveBeenCalledWith({ 'cache-control': 'no-store' });
	});
	it('submits only the review identity and session protection to Identity', async () => {
		const request = requestFor();
		expect(await actions.default(event(request) as never)).toMatchObject({
			error: null,
			result: publication
		});
		const outgoing = request.mock.calls[1][0] as Request;
		expect(outgoing.headers.get('origin')).toBe('http://localhost');
		expect(outgoing.headers.get('cookie')).toContain('hub_session=');
		expect(await outgoing.json()).toEqual({ csrf: identity.csrf, base_revision: base, digest });
	});
	it.each(['consent', 'csrf', 'digest', 'extra', 'origin', 'selection', 'oversize'])(
		'rejects %s before any GitHub write',
		async (variant) => {
			const request = requestFor({ selected: variant !== 'selection' });
			const changes: Record<string, string> =
				variant === 'consent'
					? { confirm: '' }
					: variant === 'csrf'
						? { csrf: 'forged' }
						: variant === 'digest'
							? { digest: 'bad' }
							: variant === 'extra'
								? { content: 'arbitrary' }
								: variant === 'oversize'
									? { digest: 'd'.repeat(5000) }
									: {};
			const result = await actions.default(
				event(
					request,
					changes,
					variant === 'origin' ? 'https://other.example' : 'http://localhost'
				) as never
			);
			expect(result).toHaveProperty('status');
			expect(
				request.mock.calls.every(
					([input]) =>
						!String(input instanceof Request ? input.url : input).endsWith('bootstrap-pull-request')
				)
			).toBe(true);
		}
	);
	it.each([401, 403, 409, 503])('shows safe recovery for HTTP %i', async (status) => {
		const result = await actions.default(event(requestFor({ status })) as never);
		expect(result).toMatchObject({ status, data: { result: null } });
		expect(JSON.stringify(result)).not.toContain('private-sentinel');
	});
	it('does not preview a repository outside the selected workspace', async () => {
		const request = requestFor();
		expect(await load(event(request, {}, 'http://localhost', 'other/repo') as never)).toMatchObject(
			{ preview: null }
		);
		expect(request).toHaveBeenCalledTimes(2);
	});
});
