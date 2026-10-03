import type { IssuePage } from '$lib/issues/types';

export class IssuesApiError extends Error {
	constructor(
		message: string,
		readonly status: number
	) {
		super(message);
		this.name = 'IssuesApiError';
	}
}

type Fetch = typeof fetch;

export class IssuesApiClient {
	constructor(
		private readonly baseUrl: string,
		private readonly request: Fetch
	) {}

	async listRecent(repositories: string[], page = 1, perPage = 6): Promise<IssuePage> {
		const url = this.url('/v1/issues');
		for (const repository of repositories) url.searchParams.append('repository', repository);
		url.searchParams.set('page', String(page));
		url.searchParams.set('per_page', String(perPage));

		const response = await this.request(url, {
			headers: { accept: 'application/json' },
			credentials: 'omit'
		});
		return this.read(response);
	}

	private async read(response: Response): Promise<IssuePage> {
		if (!response.ok) {
			let message = 'The issues service could not complete the request.';
			try {
				const body = (await response.json()) as { message?: unknown };
				if (typeof body.message === 'string') {
					message = body.message;
				}
			} catch {
				// Keep a stable user-facing message for malformed upstream responses.
			}
			throw new IssuesApiError(message, response.status);
		}

		const body = (await response.json()) as Partial<IssuePage>;
		if (
			!Array.isArray(body.issues) ||
			typeof body.page !== 'number' ||
			typeof body.per_page !== 'number' ||
			typeof body.has_next !== 'boolean' ||
			!Array.isArray(body.unavailable_repositories) ||
			typeof body.incomplete_details !== 'number'
		) {
			throw new IssuesApiError('The issues service returned an invalid response.', 502);
		}
		return body as IssuePage;
	}

	private url(path: string): URL {
		return new URL(path, this.baseUrl.endsWith('/') ? this.baseUrl : `${this.baseUrl}/`);
	}
}
