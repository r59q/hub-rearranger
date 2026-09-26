import type { Repository, RepositoryList } from '$lib/repositories/types';

export class RepositoriesApiError extends Error {
	constructor(
		message: string,
		readonly status: number
	) {
		super(message);
		this.name = 'RepositoriesApiError';
	}
}

type Fetch = typeof fetch;

export class RepositoriesApiClient {
	constructor(
		private readonly baseUrl: string,
		private readonly request: Fetch
	) {}

	async listSelected(): Promise<Repository[]> {
		return (await this.get('/v1/repositories')).repositories;
	}

	async listAvailable(): Promise<Repository[]> {
		return (await this.get('/v1/repositories/available')).repositories;
	}

	async replaceSelection(repositoryIds: number[]): Promise<Repository[]> {
		const response = await this.request(this.url('/v1/repository-selection'), {
			method: 'PUT',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ repository_ids: repositoryIds })
		});
		return (await this.read(response)).repositories;
	}

	private async get(path: string): Promise<RepositoryList> {
		const response = await this.request(this.url(path), {
			headers: { accept: 'application/json' }
		});
		return this.read(response);
	}

	private async read(response: Response): Promise<RepositoryList> {
		if (!response.ok) {
			let message = 'The repository service could not complete the request.';
			try {
				const body = (await response.json()) as { message?: unknown };
				if (typeof body.message === 'string') message = body.message;
			} catch {
				// Keep a stable user-facing message for malformed upstream responses.
			}
			throw new RepositoriesApiError(message, response.status);
		}

		const body = (await response.json()) as Partial<RepositoryList>;
		if (!Array.isArray(body.repositories)) {
			throw new RepositoriesApiError('The repository service returned an invalid response.', 502);
		}
		return { repositories: body.repositories };
	}

	private url(path: string): URL {
		return new URL(path, this.baseUrl.endsWith('/') ? this.baseUrl : `${this.baseUrl}/`);
	}
}
