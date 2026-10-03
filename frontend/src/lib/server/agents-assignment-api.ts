import { env } from '$env/dynamic/private';
import createClient from 'openapi-fetch';
import type { paths } from './agents-contract.gen';
import { isIssueAssignment } from './agents-assignment-response';

export async function issueAssignment(
	request: typeof fetch,
	owner: string,
	repo: string,
	number: number
) {
	const api = createClient<paths>({
		baseUrl: env.AGENTS_API_URL ?? 'http://127.0.0.1:8082',
		fetch: request,
		credentials: 'omit',
		redirect: 'manual'
	});
	const { data, response } = await api.GET(
		'/v1/repositories/{owner}/{repo}/issues/{number}/assignment',
		{ params: { path: { owner, repo, number } }, signal: AbortSignal.timeout(40_000) }
	);
	if (!response.ok || !isIssueAssignment(data, `${owner}/${repo}`, number)) {
		throw new Error(
			'Issue assignment context is unavailable. Check repository access and the Agents service, then refresh.'
		);
	}
	return data;
}
