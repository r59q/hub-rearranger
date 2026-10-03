import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import Page from './+page.svelte';
import { repository } from '../../../test/agent-fixtures';
import { assignment, csrf, review } from '../../../test/assignment-fixtures';

function data(withProposal = false) {
	return {
		repository,
		assignment: assignment(withProposal),
		identity: { state: 'authenticated', user: review.user, csrf, expiresAt: review.expires_at },
		error: null
	};
}

describe('native issue assignment view', () => {
	it('explains authority before review and preserves repository and issue in named forms', () => {
		const { body } = render(Page, { props: { data: data(), form: null } as never });
		expect(body).toContain('gpt-6.1-sol');
		expect(body).toContain('cannot merge or release');
		expect(body).toMatch(
			/action="\/issues\/assignment\?\/review(?:&amp;|&)repository=octo%2Fdemo(?:&amp;|&)number=3"/
		);
		expect(body).not.toContain('name="review_token"');
	});
	it('requires consent to exact user-attributed command and escapes issue source', () => {
		const value = data();
		value.assignment.body = '<script>alert(1)</script>';
		const { body } = render(Page, {
			props: { data: value, form: { review, error: null, reconnect: false } } as never
		});
		expect(body).toMatch(
			/action="\/issues\/assignment\?\/assign(?:&amp;|&)repository=octo%2Fdemo(?:&amp;|&)number=3"/
		);
		expect(body).toContain('name="confirm"');
		expect(body).toContain('branch-draft-pr');
		expect(body).toContain('octocat');
		expect(body).toContain('&lt;script>');
		expect(body).not.toContain('<script>alert');
	});
	it.each(['signed_out', 'reconnect_required', 'disabled', 'unavailable'])(
		'keeps activity readable without %s write identity',
		(state) => {
			const value = data(true);
			Object.assign(value.identity, { state, user: null, csrf: null });
			const { body } = render(Page, {
				props: { data: value, form: { review, error: null, reconnect: false } } as never
			});
			expect(body).toContain('Patch proposal');
			expect(body).toContain('Repository validation: unavailable');
			expect(body).not.toContain('name="confirm"');
			expect(body).not.toContain('name="review_token"');
		}
	);
	it('requires inspecting an existing assignment instead of silently starting new work', () => {
		const { body } = render(Page, { props: { data: data(true), form: null } as never });
		for (const url of [
			'https://github.com/octo/demo/pull/5',
			'https://github.com/octo/demo/runs/77',
			'https://github.com/octo/demo/tree/agent/codex-thorough/42-99'
		]) {
			expect(body).toContain(url);
		}
		expect(body).not.toContain('name="profile_revision"');
	});
});
