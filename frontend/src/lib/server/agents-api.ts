import { env } from '$env/dynamic/private';
import createClient from 'openapi-fetch';
import type { paths } from './agents-contract.gen';
import { isAssignmentConvention, type AssignmentConvention } from './agents-response';
import { isRepositoryProfiles, type RepositoryProfiles } from './agents-profiles-response';
import { isRepositoryReadiness, type RepositoryReadiness } from './agents-readiness-response';

export class AgentsApiError extends Error {
	constructor(
		message: string,
		readonly status: number
	) {
		super(message);
		this.name = 'AgentsApiError';
	}
}

export class AgentsApiClient {
	private readonly api;

	constructor(baseUrl: string, request: typeof fetch) {
		this.api = createClient<paths>({ baseUrl, fetch: request, credentials: 'omit' });
	}

	async getRepositoryProfiles(owner: string, repo: string): Promise<RepositoryProfiles> {
		try {
			const { data, response } = await this.api.GET('/v1/repositories/{owner}/{repo}/profiles', {
				params: { path: { owner, repo } },
				headers: { accept: 'application/json' },
				signal: AbortSignal.timeout(10_000)
			});
			if (!response.ok) {
				const message =
					{
						403: 'Check the Agents service token has read-only Contents access to this repository.',
						404: 'Check the repository name, default branch, and Agents service token access.',
						429: 'GitHub has rate-limited profile reads. Wait and retry.'
					}[response.status] ?? 'Repository profiles could not be loaded. Try again.';
				throw new AgentsApiError(message, response.status);
			}

			if (
				!isRepositoryProfiles(data) ||
				data.repository.toLowerCase() !== `${owner}/${repo}`.toLowerCase()
			) {
				throw new AgentsApiError('The Agents service returned an invalid response.', 502);
			}

			return data;
		} catch (error) {
			if (error instanceof AgentsApiError) {
				throw error;
			}
			if (error instanceof SyntaxError) {
				throw new AgentsApiError('The Agents service returned an invalid response.', 502);
			}
			throw new AgentsApiError('The Agents service is unavailable. Start it and try again.', 503);
		}
	}

	async getRepositoryReadiness(owner: string, repo: string): Promise<RepositoryReadiness> {
		try {
			const { data, response } = await this.api.GET('/v1/repositories/{owner}/{repo}/readiness', {
				params: { path: { owner, repo } },
				headers: { accept: 'application/json' },
				signal: AbortSignal.timeout(25_000)
			});
			if (!response.ok) {
				const message =
					{
						403: 'Check the Agents service token has read-only Metadata, Contents, and Actions access to this repository.',
						404: 'Check the repository name, default branch, and Agents service token access.',
						429: 'GitHub has rate-limited readiness reads. Wait and retry.'
					}[response.status] ?? 'Repository readiness could not be loaded. Try again.';
				throw new AgentsApiError(message, response.status);
			}

			if (
				!isRepositoryReadiness(data) ||
				data.repository.toLowerCase() !== `${owner}/${repo}`.toLowerCase()
			) {
				throw new AgentsApiError('The Agents service returned an invalid response.', 502);
			}

			return data;
		} catch (error) {
			if (error instanceof AgentsApiError) {
				throw error;
			}
			if (error instanceof SyntaxError) {
				throw new AgentsApiError('The Agents service returned an invalid response.', 502);
			}
			throw new AgentsApiError('The Agents service is unavailable. Start it and try again.', 503);
		}
	}

	async getAssignmentConvention(): Promise<AssignmentConvention> {
		try {
			const { data, response } = await this.api.GET('/v1/assignment-convention', {
				headers: { accept: 'application/json' },
				signal: AbortSignal.timeout(10_000)
			});
			if (!response.ok) {
				throw new AgentsApiError(
					'The Agents service could not complete the request. Try again.',
					response.status
				);
			}

			if (!isAssignmentConvention(data)) {
				throw new AgentsApiError('The Agents service returned an invalid response.', 502);
			}

			return data;
		} catch (error) {
			if (error instanceof AgentsApiError) {
				throw error;
			}
			if (error instanceof SyntaxError) {
				throw new AgentsApiError('The Agents service returned an invalid response.', 502);
			}
			throw new AgentsApiError('The Agents service is unavailable. Start it and try again.', 503);
		}
	}
}

export function agentsClient(request: typeof fetch): AgentsApiClient {
	return new AgentsApiClient(env.AGENTS_API_URL ?? 'http://127.0.0.1:8082', request);
}
