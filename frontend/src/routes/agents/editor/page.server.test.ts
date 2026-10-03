import { describe, expect, it, vi } from 'vitest';
import { actions, load } from './+page.server';
import { repository } from '../../../test/agent-fixtures';
import {
	base,
	digest,
	identity,
	preview,
	publication,
	editorDraft
} from '../../../test/bootstrap-fixtures';
import { transportDraft } from '$lib/server/agents-editor-api';

function requestFor(options: { selected?: boolean; status?: number; state?: string } = {}) {
	return vi.fn<typeof fetch>().mockImplementation(async (input) => {
		const request = input instanceof Request ? input : new Request(String(input));
		const path = new URL(request.url).pathname;
		if (path === '/v1/repositories') {
			return Response.json({ repositories: options.selected === false ? [] : [repository] });
		}
		if (path === '/v1/session') {
			return Response.json({
				state: 'authenticated',
				user: identity.user,
				csrf: identity.csrf,
				expires_at: identity.expiresAt
			});
		}
		if (path.endsWith('/profile-editor') && request.method === 'GET') {
			return Response.json({
				base_revision: base,
				draft: transportDraft(editorDraft()),
				diagnostics: []
			});
		}
		if (path.endsWith('/profile-editor')) {
			const value = preview();
			if (options.state === 'conflict') {
				value.state = 'conflict';
				value.diagnostics = [
					{
						code: 'INVALID_CONTEXT',
						path: '.github/agent-profiles.yml',
						message: 'Include required review context.'
					}
				];
			}
			return Response.json(value);
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

function event(request: typeof fetch, form: URLSearchParams, origin = 'http://localhost') {
	return {
		fetch: request,
		url: new URL('http://localhost/agents/editor?repository=octo%2Fdemo'),
		setHeaders: vi.fn(),
		request: new Request('http://localhost/agents/editor', {
			method: 'POST',
			headers: { origin, cookie: 'hub_session=' + 's'.repeat(43) },
			body: form
		})
	};
}

function reviewForm() {
	const draft = editorDraft();
	const form = new URLSearchParams({
		name: draft.name,
		description: draft.description,
		enabled: 'true',
		review_comments: 'true',
		setup: 'reviewed'
	});
	for (const source of draft.contextSources) {
		form.append('context_sources', source);
	}
	return form;
}

function publishForm() {
	return new URLSearchParams({
		draft: JSON.stringify(editorDraft()),
		csrf: identity.csrf!,
		base_revision: base,
		digest,
		confirm: 'reviewed'
	});
}

describe('profile editor server workflow', () => {
	it('loads fresh server choices without a durable draft', async () => {
		const input = event(requestFor(), reviewForm());
		expect(await load(input as never)).toMatchObject({
			identity,
			repository,
			editor: { draft: editorDraft(), baseRevision: base },
			error: null
		});
		expect(input.setHeaders).toHaveBeenCalledWith({ 'cache-control': 'no-store' });
	});
	it('reviews structured choices without passing session credentials to Agents', async () => {
		const request = requestFor();
		expect(await actions.review(event(request, reviewForm()) as never)).toMatchObject({
			draft: editorDraft(),
			preview: { state: 'ready' },
			result: null
		});
		const outgoing = request.mock.calls[1][0] as Request;
		expect(outgoing.method).toBe('POST');
		expect(outgoing.headers.has('cookie')).toBe(false);
		expect(outgoing.credentials).toBe('omit');
		expect(await outgoing.json()).toEqual(transportDraft(editorDraft()));
	});
	it('accepts bounded multibyte display text within the schema limit', async () => {
		const form = reviewForm();
		form.set('description', '€'.repeat(500));
		const result = await actions.review(event(requestFor(), form) as never);
		expect(result).toMatchObject({
			draft: { description: '€'.repeat(500) },
			preview: { state: 'ready' }
		});
	});
	it('returns validation conflicts with the authoring choices retained', async () => {
		expect(
			await actions.review(event(requestFor({ state: 'conflict' }), reviewForm()) as never)
		).toMatchObject({ draft: editorDraft(), preview: { state: 'conflict' }, result: null });
	});
	it('sends the review and typed choices to Identity, never file content', async () => {
		const request = requestFor();
		expect(await actions.publish(event(request, publishForm()) as never)).toMatchObject({
			result: publication
		});
		const outgoing = request.mock.calls[1][0] as Request;
		expect(await outgoing.json()).toEqual({
			csrf: identity.csrf,
			base_revision: base,
			digest,
			profile_draft: transportDraft(editorDraft())
		});
		expect(outgoing.headers.get('cookie')).toContain('hub_session=');
	});
	it.each(['setup', 'extra', 'duplicate', 'origin', 'selection', 'oversize'])(
		'rejects invalid review %s without publication',
		async (variant) => {
			const form = reviewForm();
			if (variant === 'setup') {
				form.delete('setup');
			}
			if (variant === 'extra') {
				form.set('content', 'arbitrary source');
			}
			if (variant === 'duplicate') {
				form.append('enabled', 'false');
			}
			if (variant === 'oversize') {
				form.set('description', 'x'.repeat(5000));
			}
			const request = requestFor({ selected: variant !== 'selection' });
			const result = await actions.review(
				event(
					request,
					form,
					variant === 'origin' ? 'https://other.example' : 'http://localhost'
				) as never
			);
			expect(result).toHaveProperty('status');
			expect(
				request.mock.calls.every(
					([input]) =>
						!String(input instanceof Request ? input.url : input).endsWith('/profile-editor')
				)
			).toBe(true);
		}
	);
	it.each(['consent', 'csrf', 'digest', 'draft', 'extra', 'duplicate', 'origin', 'selection'])(
		'rejects invalid publication %s before Identity',
		async (variant) => {
			const form = publishForm();
			if (variant === 'consent') {
				form.set('confirm', '');
			}
			if (variant === 'csrf') {
				form.set('csrf', 'bad');
			}
			if (variant === 'digest') {
				form.set('digest', 'bad');
			}
			if (variant === 'draft') {
				form.set('draft', '{');
			}
			if (variant === 'extra') {
				form.set('content', 'arbitrary source');
			}
			if (variant === 'duplicate') {
				form.append('digest', digest);
			}
			const request = requestFor({ selected: variant !== 'selection' });
			expect(
				await actions.publish(
					event(
						request,
						form,
						variant === 'origin' ? 'https://other.example' : 'http://localhost'
					) as never
				)
			).toHaveProperty('status');
			expect(
				request.mock.calls.every(
					([input]) =>
						!String(input instanceof Request ? input.url : input).endsWith(
							'/bootstrap-pull-request'
						)
				)
			).toBe(true);
		}
	);
	it.each([401, 403, 409, 503])(
		'retains the draft and offers safe recovery for HTTP %i',
		async (status) => {
			const result = await actions.publish(event(requestFor({ status }), publishForm()) as never);
			expect(result).toMatchObject({
				status,
				data: { draft: editorDraft(), preview: null, result: null }
			});
			expect(JSON.stringify(result)).not.toContain('private-sentinel');
		}
	);
});
