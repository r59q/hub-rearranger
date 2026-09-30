import { env } from '$env/dynamic/private';
import createClient from 'openapi-fetch';
import type { paths } from './agents-contract.gen';
import { isAssignmentConvention, type AssignmentConvention } from './agents-response';

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
		this.api = createClient<paths>({ baseUrl, fetch: request });
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
