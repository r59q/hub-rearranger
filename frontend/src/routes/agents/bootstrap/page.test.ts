import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import Page from './+page.svelte';
import { repository } from '../../../test/agent-fixtures';
import { identity, preview, publication } from '../../../test/bootstrap-fixtures';

function data() {
	return { identity, repository, preview: preview(), error: null };
}

describe('bootstrap review rendering', () => {
	it('explains every write, preserves escaped diffs and supports native forms', () => {
		const { body } = render(Page, { props: { data: data(), form: null } as never });
		for (const text of [
			'Create Git file/tree objects',
			'Create one commit',
			'Create a dedicated',
			'Create a draft PR',
			'manual',
			'subscription-backed',
			'gpt-6.1-sol/high',
			'method="POST"',
			'name="confirm"',
			'name="digest"'
		]) {
			expect(body).toContain(text);
		}
		expect(body).toContain('&lt;script>');
		expect(body).not.toContain('<script>alert(1)');
	});
	it.each(['signed_out', 'reconnect_required', 'disabled', 'unavailable'])(
		'blocks creation for %s identity',
		(state) => {
			const value = data();
			Object.assign((value.identity = { ...identity }), { state, user: null, csrf: null });
			const { body } = render(Page, { props: { data: value, form: null } as never });
			expect(body).toContain('GitHub connection');
			expect(body).not.toContain('name="confirm"');
		}
	);
	it.each(['unchanged', 'conflict'])('blocks creation for %s configuration', (state) => {
		const value = data();
		Object.assign(value.preview, { state });
		const { body } = render(Page, { props: { data: value, form: null } as never });
		expect(body).not.toContain('name="confirm"');
	});
	it('links to the verified GitHub result without another create form', () => {
		const { body } = render(Page, {
			props: { data: data(), form: { result: publication, error: null } } as never
		});
		expect(body).toContain(publication.pull_request_url);
		expect(body).toContain('Draft PR verified on GitHub');
		expect(body).not.toContain('name="confirm"');
	});
});
