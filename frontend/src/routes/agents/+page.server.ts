import { env } from '$env/dynamic/private';
import { agentsClient } from '$lib/server/agents-api';
import { loadAgentWorkspace } from '$lib/server/agents/workspace';
import { RepositoriesApiClient, RepositoriesApiError } from '$lib/server/repositories-api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url, isDataRequest, setHeaders }) => {
	setHeaders({ 'cache-control': 'no-store' });

	try {
		const repositories = (
			await new RepositoriesApiClient(
				env.REPOSITORIES_API_URL ?? 'http://127.0.0.1:8080',
				fetch
			).listSelected()
		).filter((repository) => repository.selected);

		const requested = url.searchParams.get('repository');
		const repository = requested
			? (repositories.find((repo) => repo.full_name.toLowerCase() === requested.toLowerCase()) ??
				null)
			: (repositories[0] ?? null);
		const selectionError =
			requested && !repository ? 'Choose a repository currently selected in your workspace.' : null;

		const workspace = repository
			? loadAgentWorkspace(agentsClient(fetch), repository.owner, repository.name)
			: null;

		// Initial HTML contains the full view for no-JavaScript readers. Client
		// navigation streams the caught result so slow readiness reads show loading.
		return {
			repositories,
			repository,
			selectionError,
			serviceError: null,
			workspace: isDataRequest ? workspace : await workspace
		};
	} catch (error) {
		const serviceError =
			error instanceof RepositoriesApiError
				? error.message
				: 'Repositories could not be loaded. Start the repository service and try again.';

		return {
			repositories: [],
			repository: null,
			selectionError: null,
			serviceError,
			workspace: null
		};
	}
};
