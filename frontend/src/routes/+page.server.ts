import { env } from '$env/dynamic/private';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import {
	addRepositoryToSelection,
	RepositoryNotAvailableError
} from '$lib/repositories/add-repository';
import { emptyIssuePage } from '$lib/issues/types';
import { IssuesApiClient, IssuesApiError } from '$lib/server/issues-api';
import { RepositoriesApiClient, RepositoriesApiError } from '$lib/server/repositories-api';

const repositoriesServiceUrl = env.REPOSITORIES_API_URL ?? 'http://127.0.0.1:8080';
const issuesServiceUrl = env.ISSUES_API_URL ?? 'http://127.0.0.1:8081';

function repositoriesClient(fetch: typeof globalThis.fetch): RepositoriesApiClient {
	return new RepositoriesApiClient(repositoriesServiceUrl, fetch);
}

function issuesClient(fetch: typeof globalThis.fetch): IssuesApiClient {
	return new IssuesApiClient(issuesServiceUrl, fetch);
}

function issuePageNumber(value: string | null): number {
	const page = Number(value);
	return Number.isSafeInteger(page) && page > 0 && page <= 1000 ? page : 1;
}

export const load: PageServerLoad = async ({ fetch, url }) => {
	const requestedIssuePage = issuePageNumber(url.searchParams.get('issuePage'));
	const issueViewExpanded = url.searchParams.get('issues') === 'open' || requestedIssuePage > 1;

	try {
		const availableRepositories = await repositoriesClient(fetch).listAvailable();
		const repositories = availableRepositories.filter((repository) => repository.selected);
		if (repositories.length === 0) {
			return {
				repositories,
				availableRepositories,
				serviceError: null,
				issuePage: emptyIssuePage(requestedIssuePage),
				issueError: null,
				issueViewExpanded
			};
		}

		try {
			const issuePage = await issuesClient(fetch).listRecent(
				repositories.map((repository) => repository.full_name),
				requestedIssuePage
			);
			return {
				repositories,
				availableRepositories,
				serviceError: null,
				issuePage,
				issueError: null,
				issueViewExpanded
			};
		} catch (error) {
			const issueError =
				error instanceof IssuesApiError
					? error.message
					: 'The issues service is unavailable. Start it and try again.';
			return {
				repositories,
				availableRepositories,
				serviceError: null,
				issuePage: emptyIssuePage(requestedIssuePage),
				issueError,
				issueViewExpanded
			};
		}
	} catch (error) {
		const message =
			error instanceof RepositoriesApiError
				? error.message
				: 'The repository service is unavailable. Start it and try again.';
		return {
			repositories: [],
			availableRepositories: [],
			serviceError: message,
			issuePage: emptyIssuePage(requestedIssuePage),
			issueError: null,
			issueViewExpanded
		};
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
			await addRepositoryToSelection(repositoriesClient(fetch), repositoryId);
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
