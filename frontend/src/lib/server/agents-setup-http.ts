import { env } from '$env/dynamic/private';
import type { RequestEvent } from '@sveltejs/kit';
import { RepositoriesApiClient } from './repositories-api';
import { IdentityApiError } from './identity-api';

export async function selectedRepository(event: Pick<RequestEvent, 'fetch' | 'url'>) {
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

export async function boundedForm(request: Request, maximumBytes = 4096): Promise<string> {
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
			if (length > maximumBytes) {
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
