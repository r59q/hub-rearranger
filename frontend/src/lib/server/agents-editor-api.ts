import { env } from '$env/dynamic/private';
import createClient from 'openapi-fetch';
import type { paths, components } from './agents-contract.gen';
import { isEditorDraft, type EditorDraft } from '$lib/agents/editor';
import { bootstrapView, type BootstrapView } from './agents-bootstrap-api';

export function transportDraft(draft: EditorDraft): components['schemas']['ProfileDraft'] {
	return {
		name: draft.name,
		description: draft.description,
		enabled: draft.enabled,
		context_sources:
			draft.contextSources as components['schemas']['ProfileDraft']['context_sources'],
		review_comments: draft.reviewComments
	};
}

function client(request: typeof fetch) {
	return createClient<paths>({
		baseUrl: env.AGENTS_API_URL ?? 'http://127.0.0.1:8082',
		fetch: request,
		redirect: 'manual',
		credentials: 'omit'
	});
}

export async function profileEditor(request: typeof fetch, owner: string, repo: string) {
	const { data, response } = await client(request).GET(
		'/v1/repositories/{owner}/{repo}/profile-editor',
		{
			params: { path: { owner, repo } },
			signal: AbortSignal.timeout(50_000)
		}
	);
	if (
		!response.ok ||
		!data ||
		Object.keys(data).some((key) => !['base_revision', 'draft', 'diagnostics'].includes(key)) ||
		!/^[0-9a-f]{40}$/.test(data.base_revision) ||
		!Array.isArray(data.diagnostics) ||
		(data.draft !== null &&
			(!data.draft ||
				Object.keys(data.draft).length !== 5 ||
				Object.keys(data.draft).some(
					(key) =>
						!['name', 'description', 'enabled', 'context_sources', 'review_comments'].includes(key)
				))) ||
		data.diagnostics.some(
			(row) =>
				!row ||
				Object.keys(row).length !== 3 ||
				['code', 'path', 'message'].some((key) => typeof row[key as keyof typeof row] !== 'string')
		)
	) {
		throw new Error('Profile choices are unavailable. Check repository access and reload.');
	}
	const draft: EditorDraft | null = data.draft
		? {
				name: data.draft.name,
				description: data.draft.description,
				enabled: data.draft.enabled,
				contextSources: data.draft.context_sources,
				reviewComments: data.draft.review_comments
			}
		: null;
	if (
		(draft && (!isEditorDraft(draft) || data.diagnostics.length > 0)) ||
		(!draft && data.diagnostics.length === 0)
	) {
		throw new Error('Profile choices are unavailable. Check repository access and reload.');
	}
	return { draft, diagnostics: data.diagnostics, baseRevision: data.base_revision };
}

export async function previewProfile(
	request: typeof fetch,
	owner: string,
	repo: string,
	draft: EditorDraft
): Promise<BootstrapView> {
	const { data, response } = await client(request).POST(
		'/v1/repositories/{owner}/{repo}/profile-editor',
		{
			params: { path: { owner, repo } },
			body: transportDraft(draft),
			signal: AbortSignal.timeout(50_000)
		}
	);
	if (!response.ok) {
		throw new Error('Profile review is unavailable. Check repository access and reload.');
	}
	return bootstrapView(data, `${owner}/${repo}`);
}
