import { env } from '$env/dynamic/private';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { RepositoriesApiClient, RepositoriesApiError } from '$lib/server/repositories-api';

const serviceUrl = env.REPOSITORIES_API_URL ?? 'http://127.0.0.1:8080';

function client(fetch: typeof globalThis.fetch): RepositoriesApiClient {
	return new RepositoriesApiClient(serviceUrl, fetch);
}

export const load: PageServerLoad = async ({ fetch }) => {
	try {
		const availableRepositories = await client(fetch).listAvailable();
		const repositories = availableRepositories.filter((repository) => repository.selected);
		return { repositories, availableRepositories, serviceError: null };
	} catch (error) {
		const message =
			error instanceof RepositoriesApiError
				? error.message
				: 'The repository service is unavailable. Start it and try again.';
		return { repositories: [], availableRepositories: [], serviceError: message };
	}
};

export const actions = {
	default: async ({ request, fetch }) => {
		const formData = await request.formData();
		const values = formData.getAll('repositoryId');
		const repositoryIds = values.map(Number);
		if (repositoryIds.some((id) => !Number.isSafeInteger(id) || id <= 0)) {
			return fail(400, { success: false, message: 'The repository selection was invalid.' });
		}

		try {
			await client(fetch).replaceSelection(repositoryIds);
			return { success: true, message: 'Repository selection saved.' };
		} catch (error) {
			const message =
				error instanceof RepositoriesApiError
					? error.message
					: 'The repository selection could not be saved. Try again.';
			return fail(error instanceof RepositoriesApiError ? error.status : 500, {
				success: false,
				message
			});
		}
	}
} satisfies Actions;
