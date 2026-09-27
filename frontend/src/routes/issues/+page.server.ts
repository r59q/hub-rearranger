import { env } from '$env/dynamic/private';
import { emptyIssuePage } from '$lib/issues/types';
import { IssuesApiClient, IssuesApiError } from '$lib/server/issues-api';
import { RepositoriesApiClient, RepositoriesApiError } from '$lib/server/repositories-api';
import type { PageServerLoad } from './$types';

const repositoriesServiceUrl = env.REPOSITORIES_API_URL ?? 'http://127.0.0.1:8080';
const issuesServiceUrl = env.ISSUES_API_URL ?? 'http://127.0.0.1:8081';

function pageNumber(value: string | null): number {
	const page = Number(value);
	return Number.isSafeInteger(page) && page > 0 && page <= 1000 ? page : 1;
}

export const load: PageServerLoad = async ({ fetch, url }) => {
	const requestedPage = pageNumber(url.searchParams.get('issuePage'));
	const repositoriesClient = new RepositoriesApiClient(repositoriesServiceUrl, fetch);

	try {
		const repositories = (await repositoriesClient.listAvailable()).filter(
			(repository) => repository.selected
		);
		if (repositories.length === 0) {
			return {
				repositories,
				serviceError: null,
				issuePage: emptyIssuePage(requestedPage),
				issueError: null
			};
		}

		try {
			const issuePage = await new IssuesApiClient(issuesServiceUrl, fetch).listRecent(
				repositories.map((repository) => repository.full_name),
				requestedPage
			);
			return { repositories, serviceError: null, issuePage, issueError: null };
		} catch (error) {
			const issueError =
				error instanceof IssuesApiError
					? error.message
					: 'The issues service is unavailable. Start it and try again.';
			return {
				repositories,
				serviceError: null,
				issuePage: emptyIssuePage(requestedPage),
				issueError
			};
		}
	} catch (error) {
		const serviceError =
			error instanceof RepositoriesApiError
				? error.message
				: 'The repository service is unavailable. Start it and try again.';
		return {
			repositories: [],
			serviceError,
			issuePage: emptyIssuePage(requestedPage),
			issueError: null
		};
	}
};
