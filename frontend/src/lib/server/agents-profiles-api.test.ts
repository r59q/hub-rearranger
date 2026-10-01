import { describe, expect, it, vi } from 'vitest';
import { AgentsApiClient } from './agents-api';
import type { RepositoryProfiles } from './agents-profiles-response';

const revision = '0123456789abcdef0123456789abcdef01234567';
const catalog = {
	repository: 'octo/demo',
	catalog_path: '.github/agent-profiles.yml',
	default_branch: 'main',
	revision,
	state: 'valid',
	profiles: [
		{
			id: 'codex-thorough',
			revision,
			configuration: {
				enabled: true,
				name: 'Codex Thorough',
				description: 'Implement a repository issue.',
				role: 'implementation',
				adapter: {
					id: 'codex-chatgpt-private-runner',
					contract_version: 1,
					runner_label: 'hub-agent-codex'
				},
				model: { id: 'gpt-6.1-sol', reasoning_effort: 'high', fallback: 'none' },
				triggers: { assignment: 'issue_comment.created' },
				context: { sources: ['issue', 'repository'], images: false },
				authority: { mode: 'branch-draft-pr', sandbox: 'workspace-write', network: false },
				validation: { checks: ['repository-check'], on_failure: 'draft-with-evidence' },
				continuation: { review_comments: false, pipeline: 'disabled' }
			}
		}
	],
	diagnostics: []
} satisfies RepositoryProfiles;

function clientFor(body: unknown, status = 200) {
	const request = vi.fn<typeof fetch>().mockResolvedValue(Response.json(body, { status }));
	return { client: new AgentsApiClient('http://agents:8082', request), request };
}

describe('repository profile adapter', () => {
	it('reads the selected repository through the server API with no browser credentials', async () => {
		const { client, request } = clientFor(catalog);

		const result = await client.getRepositoryProfiles('octo', 'demo');

		expect(result).toEqual(catalog);
		const outgoing = request.mock.calls[0][0] as Request;
		expect(outgoing.url).toBe('http://agents:8082/v1/repositories/octo/demo/profiles');
		expect(outgoing.headers.has('authorization')).toBe(false);
	});

	it.each(['missing', 'invalid', 'unsupported'] as const)(
		'preserves actionable %s states',
		async (state) => {
			const body = {
				...catalog,
				state,
				profiles: [],
				diagnostics: [
					{
						code: {
							missing: 'MISSING_FILE',
							invalid: 'INVALID_YAML',
							unsupported: 'UNSUPPORTED_VERSION'
						}[state],
						path: '/',
						message: 'Fix the catalog.'
					}
				]
			};
			const { client } = clientFor(body);

			expect(await client.getRepositoryProfiles('octo', 'demo')).toEqual(body);
		}
	);

	it.each([
		{},
		{ ...catalog, repository: 'other/repo' },
		{ ...catalog, revision: 'main' },
		{ ...catalog, state: 'runtime-verified' },
		{ ...catalog, profiles: [] },
		{ ...catalog, profiles: [{ ...catalog.profiles[0], revision: 'f'.repeat(40) }] },
		{ ...catalog, profiles: [catalog.profiles[0], catalog.profiles[0]] },
		{ ...catalog, state: 'invalid' },
		{
			...catalog,
			state: 'unsupported',
			profiles: [],
			diagnostics: [{ code: 'MISSING_FILE', path: '/', message: 'Fix the catalog.' }]
		},
		{ ...catalog, diagnostics: [{ code: 'unsupported', path: '/', message: 'private-sentinel' }] },
		{
			...catalog,
			profiles: [
				{
					...catalog.profiles[0],
					configuration: { ...catalog.profiles[0].configuration, command: 'private-sentinel' }
				}
			]
		},
		{
			...catalog,
			profiles: [
				{
					...catalog.profiles[0],
					configuration: {
						...catalog.profiles[0].configuration,
						authority: { mode: 'merge', sandbox: 'workspace-write', network: false }
					}
				}
			]
		},
		{
			...catalog,
			profiles: [
				{
					...catalog.profiles[0],
					configuration: {
						...catalog.profiles[0].configuration,
						continuation: { review_comments: true, pipeline: 'disabled' }
					}
				}
			]
		}
	])('rejects malformed, unsafe, or inconsistent responses', async (body) => {
		const { client } = clientFor(body);

		await expect(client.getRepositoryProfiles('octo', 'demo')).rejects.toMatchObject({
			status: 502,
			message: 'The Agents service returned an invalid response.'
		});
	});

	it.each([403, 404, 429, 502, 500])(
		'reports HTTP %i without exposing upstream text',
		async (status) => {
			const { client } = clientFor({ message: 'private-sentinel' }, status);

			await expect(client.getRepositoryProfiles('octo', 'demo')).rejects.toMatchObject({ status });
			await expect(client.getRepositoryProfiles('octo', 'demo')).rejects.not.toHaveProperty(
				'message',
				'private-sentinel'
			);
		}
	);

	it('reports invalid JSON safely', async () => {
		const request = vi.fn<typeof fetch>().mockResolvedValue(new Response('private-sentinel {'));
		const client = new AgentsApiClient('http://agents:8082', request);

		await expect(client.getRepositoryProfiles('octo', 'demo')).rejects.toMatchObject({
			status: 502
		});
	});

	it('reports network failure safely', async () => {
		const request = vi.fn<typeof fetch>().mockRejectedValue(new Error('private-sentinel'));
		const client = new AgentsApiClient('http://agents:8082', request);

		await expect(client.getRepositoryProfiles('octo', 'demo')).rejects.toMatchObject({
			status: 503
		});
	});
});
