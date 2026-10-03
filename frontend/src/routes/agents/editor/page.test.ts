import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import Page from './+page.svelte';
import { repository } from '../../../test/agent-fixtures';
import {
	base,
	identity,
	preview,
	publication,
	editorDraft
} from '../../../test/bootstrap-fixtures';

function data() {
	return {
		repository,
		identity,
		editor: { draft: editorDraft(), diagnostics: [], baseRevision: base },
		error: null
	};
}

describe('profile editor rendering', () => {
	it('exposes supported policy and setup requirements in a native review form', () => {
		const { body } = render(Page, { props: { data: data(), form: null } as never });
		for (const text of [
			'name="name"',
			'name="context_sources"',
			'name="setup"',
			'?/review',
			'gpt-6.1-sol/high',
			'branch-draft-pr',
			'repository-check',
			'Image context: disabled',
			'Pipeline events',
			'Runner setup tutorial',
			'subscription',
			'browser'
		]) {
			expect(body).toContain(text);
		}
		expect(body).not.toContain('name="confirm"');
		expect(body).toMatch(/action="\?\/review(?:&amp;|&)repository=octo%2Fdemo"/);
	});
	it('renders a server review and publication form without needing client effects', () => {
		const { body } = render(Page, {
			props: {
				data: data(),
				form: { draft: editorDraft(), preview: preview(), result: null, error: null }
			} as never
		});
		expect(body).toContain('name="confirm"');
		expect(body).toMatch(/action="\?\/publish(?:&amp;|&)repository=octo%2Fdemo"/);
		expect(body).toContain('&lt;script>');
		expect(body).not.toContain('<script>alert(1)');
	});
	it.each(['conflict', 'unchanged'])('blocks publication for %s review', (state) => {
		const review = preview();
		Object.assign(review, { state });
		const { body } = render(Page, {
			props: {
				data: data(),
				form: { draft: editorDraft(), preview: review, result: null, error: null }
			} as never
		});
		expect(body).not.toContain('name="confirm"');
	});
	it.each(['disabled', 'signed_out', 'reconnect_required', 'unavailable'])(
		'blocks publication when identity is %s',
		(state) => {
			const value = data();
			Object.assign((value.identity = { ...identity }), { state, csrf: null, user: null });
			const { body } = render(Page, {
				props: {
					data: value,
					form: { draft: editorDraft(), preview: preview(), result: null, error: null }
				} as never
			});
			expect(body).toContain('GitHub connection');
			expect(body).not.toContain('name="confirm"');
		}
	);
	it('links a verified PR and removes authoring controls after publication', () => {
		const { body } = render(Page, {
			props: {
				data: data(),
				form: { draft: editorDraft(), preview: null, result: publication, error: null }
			} as never
		});
		expect(body).toContain(publication.pull_request_url);
		expect(body).toContain('Draft PR verified');
		expect(body).not.toContain('name="setup"');
	});
});
