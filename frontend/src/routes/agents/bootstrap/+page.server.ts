import { env } from '$env/dynamic/private';
import { fail } from '@sveltejs/kit';
import { bootstrapPreview } from '$lib/server/agents-bootstrap-api';
import { identityFor, sameOrigin } from '$lib/server/identity-http';
import { IdentityApiError } from '$lib/server/identity-api';
import { RepositoriesApiClient } from '$lib/server/repositories-api';
import type { Actions, PageServerLoad } from './$types';
import type { RequestEvent } from '@sveltejs/kit';

async function selectedRepository(event: Pick<RequestEvent, 'fetch' | 'url'>) {
	const repositories = await new RepositoriesApiClient(
		env.REPOSITORIES_API_URL ?? 'http://127.0.0.1:8080',
		event.fetch
	).listSelected();
	const name = event.url.searchParams.get('repository');
	return (
		repositories.find(
			(repo) => repo.selected && repo.full_name.toLowerCase() === name?.toLowerCase()
		) ?? null
	);
}

export const load: PageServerLoad = async (event) => {
	event.setHeaders({ 'cache-control': 'no-store' });
	const identity = await identityFor(event).session();
	try {
		const repository = await selectedRepository(event);
		if (!repository) {
			return {
				identity,
				repository: null,
				preview: null,
				error: 'Choose a repository in your workspace, then open its bootstrap review.'
			};
		}
		const preview = await bootstrapPreview(event.fetch, repository.owner, repository.name);
		return { identity, repository, preview, error: null };
	} catch {
		return {
			identity,
			repository: null,
			preview: null,
			error:
				'Bootstrap review is unavailable. Check repository access and the Agents service, then reload.'
		};
	}
};

async function boundedForm(request: Request): Promise<string> {
	const reader = request.body?.getReader();
	if (!reader) {
		throw new Error('Missing review');
	}
	const chunks: Uint8Array[] = [];
	let length = 0;
	try {
		while (true) {
			const { done, value } = await reader.read();
			if (done) {
				break;
			}
			length += value.length;
			if (length > 4096) {
				await reader.cancel();
				throw new IdentityApiError(400);
			}
			chunks.push(value);
		}
	} finally {
		reader.releaseLock();
	}
	return new TextDecoder().decode(Buffer.concat(chunks));
}

export const actions: Actions = {
	default: async (event) => {
		if (!sameOrigin(event)) {
			return fail(403, {
				error: 'The request could not be verified. Reload before trying again.',
				result: null
			});
		}
		try {
			// A bounded, closed form carries only consent and review identity.
			const text = await boundedForm(event.request);
			if (
				text.length > 4096 ||
				!event.request.headers.get('content-type')?.startsWith('application/x-www-form-urlencoded')
			) {
				return fail(400, {
					error: 'The review request is invalid. Reload before trying again.',
					result: null
				});
			}
			const form = new URLSearchParams(text);
			if (
				[...form.keys()].some(
					(key) => !['csrf', 'base_revision', 'digest', 'confirm'].includes(key)
				) ||
				['csrf', 'base_revision', 'digest', 'confirm'].some(
					(key) => form.getAll(key).length !== 1
				) ||
				!/^[A-Za-z0-9_-]{43}$/.test(form.get('csrf') ?? '') ||
				!/^[0-9a-f]{40}$/.test(form.get('base_revision') ?? '') ||
				!/^[0-9a-f]{64}$/.test(form.get('digest') ?? '') ||
				form.get('confirm') !== 'reviewed'
			) {
				return fail(400, {
					error: 'Review the changes and confirm the GitHub writes before creating a draft PR.',
					result: null
				});
			}
			const repository = await selectedRepository(event);
			if (!repository) {
				return fail(400, {
					error: 'This repository is no longer selected. Return to the Agents view.',
					result: null
				});
			}
			const result = await identityFor(event).createBootstrap(
				repository.owner,
				repository.name,
				form.get('csrf')!,
				form.get('base_revision')!,
				form.get('digest')!
			);
			return { error: null, result };
		} catch (error) {
			const status = error instanceof IdentityApiError ? error.status : 503;
			return fail(status >= 400 && status < 600 ? status : 503, {
				error:
					error instanceof IdentityApiError
						? error.message
						: 'Bootstrap publication is unavailable. Reload and review again to reconcile any existing GitHub result.',
				result: null
			});
		}
	}
};
