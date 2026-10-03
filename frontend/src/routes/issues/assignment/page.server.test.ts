import { describe, expect, it, vi } from 'vitest';
import { actions, load } from './+page.server';
import { repository } from '../../../test/agent-fixtures';
import {
	assignment,
	assignmentResult,
	csrf,
	review,
	revision
} from '../../../test/assignment-fixtures';

function requestFor(
	options: {
		status?: number;
		selected?: boolean;
		identityState?: string;
		proposal?: boolean;
		lost?: boolean;
	} = {}
) {
	return vi.fn<typeof fetch>().mockImplementation(async (input) => {
		const request = input instanceof Request ? input : new Request(String(input));
		const path = new URL(request.url).pathname;
		if (path === '/v1/repositories') {
			return Response.json({ repositories: options.selected === false ? [] : [repository] });
		}
		if (path === '/v1/session') {
			return Response.json({
				state: options.identityState ?? 'authenticated',
				user: options.identityState === 'signed_out' ? null : review.user,
				csrf: options.identityState === 'signed_out' ? null : csrf,
				expires_at: options.identityState === 'signed_out' ? null : review.expires_at
			});
		}
		if (request.method === 'GET' && path.endsWith('/assignment')) {
			return Response.json(assignment(options.proposal));
		}
		if (path.endsWith('/assignment-review')) {
			return Response.json(
				options.status ? { code: 'assignment_stale', message: 'private-sentinel' } : review,
				{ status: options.status ?? 200 }
			);
		}
		if (path.endsWith('/assignment')) {
			if (options.lost) {
				throw new Error('lost response private-sentinel');
			}
			return Response.json(
				options.status
					? { code: 'assignment_uncertain', message: 'private-sentinel' }
					: assignmentResult,
				{ status: options.status ?? 200 }
			);
		}
		return new Response(null, { status: 404 });
	});
}

function event(
	fetch: typeof globalThis.fetch,
	fields: Record<string, string> = { csrf, profile_revision: revision },
	origin = 'http://localhost',
	selected = 'octo/demo'
) {
	const url = new URL(
		'http://localhost/issues/assignment?' +
			new URLSearchParams({ repository: selected, number: '3' })
	);
	return {
		fetch,
		url,
		setHeaders: vi.fn(),
		request: new Request(url, {
			method: 'POST',
			headers: { origin, cookie: 'hub_session=' + 's'.repeat(43) + '; unrelated=private-sentinel' },
			body: new URLSearchParams(fields)
		})
	};
}

describe('issue assignment server workflow', () => {
	it('loads issue context and GitHub history when signed out', async () => {
		const input = event(requestFor({ identityState: 'signed_out', proposal: true }));
		expect(await load(input as never)).toMatchObject({
			identity: { state: 'signed_out' },
			assignment: assignment(true),
			error: null
		});
		expect(input.setHeaders).toHaveBeenCalledWith({ 'cache-control': 'no-store' });
	});
	it('gets user-attributed one-use review without a GitHub write', async () => {
		const request = requestFor();
		expect(await actions.review(event(request) as never)).toMatchObject({ review, error: null });
		const outgoing = request.mock.calls[1][0] as Request;
		expect(JSON.parse(await outgoing.text())).toEqual({ csrf, profile_revision: revision });
		expect(outgoing.headers.get('cookie')).toBe('hub_session=' + 's'.repeat(43));
		expect(outgoing.headers.has('authorization')).toBe(false);
	});
	it('posts only consent identity and redirects to fresh GitHub state', async () => {
		const request = requestFor();
		await expect(
			actions.assign(
				event(request, { csrf, review_token: review.review_token, confirm: 'reviewed' }) as never
			)
		).rejects.toMatchObject({
			status: 303,
			location: '/issues/assignment?repository=octo%2Fdemo&number=3'
		});
		const outgoing = request.mock.calls[1][0] as Request;
		expect(JSON.parse(await outgoing.text())).toEqual({ csrf, review_token: review.review_token });
		expect(request.mock.calls.length).toBe(2);
	});
	it.each([401, 403, 409, 503])('shows safe recovery for %s and never retries', async (status) => {
		const request = requestFor({ status });
		const result = await actions.assign(
			event(request, { csrf, review_token: review.review_token, confirm: 'reviewed' }) as never
		);
		expect(result).toMatchObject({
			status: status >= 500 ? 409 : status,
			data: { review: null, reconnect: status === 401 }
		});
		if (status >= 500) {
			expect(JSON.stringify(result)).toContain('may have posted');
		}
		expect(JSON.stringify(result)).not.toContain('private-sentinel');
		expect(request.mock.calls.length).toBe(2);
	});
	it('keeps a lost write response uncertain without retrying', async () => {
		const request = requestFor({ lost: true });
		const result = await actions.assign(
			event(request, { csrf, review_token: review.review_token, confirm: 'reviewed' }) as never
		);
		expect(result).toMatchObject({ status: 409, data: { review: null } });
		expect(JSON.stringify(result)).toContain('may have posted');
		expect(request.mock.calls.length).toBe(2);
	});
	it.each(['origin', 'extra', 'consent', 'csrf', 'unselected'])(
		'rejects invalid %s before writing',
		async (variant) => {
			const request = requestFor({ selected: variant !== 'unselected' });
			const fields: Record<string, string> = {
				csrf,
				review_token: review.review_token,
				confirm: 'reviewed'
			};
			if (variant === 'extra') {
				fields.command = 'arbitrary';
			}
			if (variant === 'consent') {
				fields.confirm = 'no';
			}
			if (variant === 'csrf') {
				fields.csrf = 'forged';
			}
			const result = await actions.assign(
				event(
					request,
					fields,
					variant === 'origin' ? 'https://foreign.example' : 'http://localhost'
				) as never
			);
			expect(result).toMatchObject({ status: variant === 'origin' ? 403 : 400 });
			expect(request.mock.calls.length).toBe(variant === 'unselected' ? 1 : 0);
		}
	);
});
