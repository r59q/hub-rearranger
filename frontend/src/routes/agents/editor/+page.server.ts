import { fail } from '@sveltejs/kit';
import { profileEditor, previewProfile, transportDraft } from '$lib/server/agents-editor-api';
import { boundedForm, selectedRepository } from '$lib/server/agents-setup-http';
import { identityFor, sameOrigin } from '$lib/server/identity-http';
import { IdentityApiError } from '$lib/server/identity-api';
import { isEditorDraft, type EditorDraft } from '$lib/agents/editor';
import type { Actions, PageServerLoad } from './$types';
import type { RequestEvent } from '@sveltejs/kit';

export const load: PageServerLoad = async (event) => {
	event.setHeaders({ 'cache-control': 'no-store' });
	const identity = await identityFor(event).session();
	try {
		const repository = await selectedRepository(event);
		if (!repository) {
			return {
				identity,
				repository: null,
				editor: null,
				error: 'Choose a repository in your workspace, then open its profile editor.'
			};
		}
		const editor = await profileEditor(event.fetch, repository.owner, repository.name);
		return { identity, repository, editor, error: null };
	} catch {
		return {
			identity,
			repository: null,
			editor: null,
			error:
				'Profile editing is unavailable. Check repository access and the Agents service, then reload.'
		};
	}
};

async function editorForm(event: RequestEvent, allowed: string[], repeated: string[] = []) {
	if (!sameOrigin(event)) {
		throw new IdentityApiError(403);
	}
	if (!event.request.headers.get('content-type')?.startsWith('application/x-www-form-urlencoded')) {
		throw new IdentityApiError(400);
	}
	const form = new URLSearchParams(await boundedForm(event.request, 16384));
	if (
		[...form.keys()].some((key) => !allowed.includes(key)) ||
		allowed.some((key) => !repeated.includes(key) && form.getAll(key).length !== 1)
	) {
		throw new IdentityApiError(400);
	}
	return form;
}

function draftFromForm(form: URLSearchParams): EditorDraft {
	if (
		!['true', 'false'].includes(form.get('enabled') ?? '') ||
		!['true', 'false'].includes(form.get('review_comments') ?? '') ||
		form.get('setup') !== 'reviewed'
	) {
		throw new IdentityApiError(400);
	}
	const draft = {
		name: form.get('name')!,
		description: form.get('description')!,
		enabled: form.get('enabled') === 'true',
		contextSources: form.getAll('context_sources'),
		reviewComments: form.get('review_comments') === 'true'
	};
	if (!isEditorDraft(draft)) {
		throw new IdentityApiError(400);
	}
	return draft;
}

function failure(error: unknown, draft: EditorDraft | null) {
	const status = error instanceof IdentityApiError ? error.status : 503;
	return fail(status, {
		error:
			status === 400
				? 'Check the profile choices and acknowledge the setup requirements before reviewing or publishing.'
				: error instanceof IdentityApiError
					? error.message
					: 'Profile review is unavailable. Keep your browser draft and retry when Agents is available.',
		draft,
		preview: null,
		result: null
	});
}

export const actions: Actions = {
	review: async (event) => {
		let draft: EditorDraft | null = null;
		try {
			const form = await editorForm(
				event,
				['name', 'description', 'enabled', 'context_sources', 'review_comments', 'setup'],
				['context_sources']
			);
			draft = draftFromForm(form);
			const repository = await selectedRepository(event);
			if (!repository) {
				throw new IdentityApiError(400);
			}
			const preview = await previewProfile(event.fetch, repository.owner, repository.name, draft);
			return { draft, preview, error: null, result: null };
		} catch (error) {
			return failure(error, draft);
		}
	},
	publish: async (event) => {
		let draft: EditorDraft | null = null;
		try {
			const form = await editorForm(event, ['draft', 'csrf', 'base_revision', 'digest', 'confirm']);
			const parsed: unknown = JSON.parse(form.get('draft')!);
			if (
				!isEditorDraft(parsed) ||
				!/^[A-Za-z0-9_-]{43}$/.test(form.get('csrf')!) ||
				!/^[0-9a-f]{40}$/.test(form.get('base_revision')!) ||
				!/^[0-9a-f]{64}$/.test(form.get('digest')!) ||
				form.get('confirm') !== 'reviewed'
			) {
				throw new IdentityApiError(400);
			}
			draft = parsed;
			const repository = await selectedRepository(event);
			if (!repository) {
				throw new IdentityApiError(400);
			}
			const result = await identityFor(event).createBootstrap(
				repository.owner,
				repository.name,
				form.get('csrf')!,
				form.get('base_revision')!,
				form.get('digest')!,
				transportDraft(draft)
			);
			return { draft, preview: null, result, error: null };
		} catch (error) {
			return failure(error instanceof SyntaxError ? new IdentityApiError(400) : error, draft);
		}
	}
};
