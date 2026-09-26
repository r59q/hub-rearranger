import { env } from '$env/dynamic/private';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import {
	addRepositoryToSelection,
	RepositoryNotAvailableError
} from '$lib/repositories/add-repository';
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
	addRepository: async ({ request, fetch }) => {
		const formData = await request.formData();
		const repositoryId = Number(formData.get('repositoryId'));
		if (!Number.isSafeInteger(repositoryId) || repositoryId <= 0) {
			return fail(400, { success: false, message: 'The repository could not be added.' });
		}

		try {
			await addRepositoryToSelection(client(fetch), repositoryId);
			return { success: true, message: 'Repository added to the workspace.' };
		} catch (error) {
			if (error instanceof RepositoryNotAvailableError) {
				return fail(422, { success: false, message: error.message });
			}
			const message =
				error instanceof RepositoriesApiError
					? error.message
					: 'The repository could not be added. Try again.';
			return fail(error instanceof RepositoriesApiError ? error.status : 500, {
				success: false,
				message
			});
		}
	}
} satisfies Actions;
