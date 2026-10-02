import { describe, expect, it, vi } from 'vitest';
import { AgentsApiClient } from './agents-api';
import type { RepositoryReadiness } from './agents-readiness-response';

const revision = 'a'.repeat(40);
const evidence = {
	version: 1,
	repository: 'octo/demo',
	profile_id: 'codex-thorough',
	profile_revision: revision,
	runner_label: 'hub-agent-codex',
	requested_model: 'gpt-6.1-sol',
	requested_reasoning_effort: 'high',
	effective_model: 'gpt-6.1-sol',
	effective_reasoning_effort: 'high',
	cli_version: 'codex-cli 0.157.1',
	run_id: 10,
	run_attempt: 2,
	verified_at: '2026-10-01T12:00:00Z',
	outcome: 'verified',
	reason_code: 'verified'
} as const;
const body = {
	repository: 'octo/demo',
	default_branch: 'main',
	revision,
	catalog_state: 'valid',
	diagnostics: [],
	profiles: [
		{
			id: 'codex-thorough',
			revision,
			runner_label: 'hub-agent-codex',
			state: 'runtime_verified',
			next_action: 'Refresh the diagnostic within 24 hours.',
			diagnostics: [],
			evidence
		}
	]
} satisfies RepositoryReadiness;

function clientFor(value: unknown, status = 200) {
	const request = vi
		.fn<typeof fetch>()
		.mockImplementation(async () => Response.json(value, { status }));
	return { client: new AgentsApiClient('http://agents:8082', request), request };
}

describe('readiness API adapter', () => {
	it('reads safe evidence through the server-only API', async () => {
		const { client, request } = clientFor(body);
		expect(await client.getRepositoryReadiness('octo', 'demo')).toEqual(body);

		const outgoing = request.mock.calls[0][0] as Request;
		expect(outgoing.url).toBe('http://agents:8082/v1/repositories/octo/demo/readiness');
		expect(outgoing.headers.has('authorization')).toBe(false);
	});

	it.each(['configuration_missing', 'verification_pending', 'verification_failed'] as const)(
		'preserves %s and its next action',
		async (state) => {
			const row = {
				...body.profiles[0],
				state,
				diagnostics: [
					{
						code: 'EVIDENCE_MISSING',
						path: '.github/workflows/agent-profile-diagnostic.yml',
						message: 'Run the diagnostic.'
					}
				],
				evidence:
					state === 'verification_failed'
						? {
								...evidence,
								outcome: 'failed',
								reason_code: 'request_failed',
								effective_model: null,
								effective_reasoning_effort: null
							}
						: undefined
			};
			const value = { ...body, profiles: [row] };
			const { client } = clientFor(value);
			expect(await client.getRepositoryReadiness('octo', 'demo')).toEqual(value);
		}
	);

	it.each([
		{},
		{ ...body, repository: 'other/repo' },
		{ ...body, revision: 'main' },
		{ ...body, profiles: [] },
		{ ...body, profiles: [body.profiles[0], body.profiles[0]] },
		{ ...body, catalog_state: 'invalid' },
		{
			...body,
			profiles: [
				{ ...body.profiles[0], evidence: { ...evidence, verified_at: '2026-02-30T12:00:00Z' } }
			]
		},
		{ ...body, profiles: [{ ...body.profiles[0], state: 'verification_pending' }] },
		{
			...body,
			profiles: [{ ...body.profiles[0], evidence: { ...evidence, stderr: 'private-sentinel' } }]
		},
		{
			...body,
			profiles: [{ ...body.profiles[0], evidence: { ...evidence, effective_model: null } }]
		},
		{
			...body,
			profiles: [
				{ ...body.profiles[0], evidence: { ...evidence, profile_revision: 'b'.repeat(40) } }
			]
		},
		{
			...body,
			profiles: [{ ...body.profiles[0], evidence: { ...evidence, runner_label: 'addons' } }]
		},
		{
			...body,
			profiles: [
				{ ...body.profiles[0], evidence: { ...evidence, cli_version: 'private-sentinel' } }
			]
		},
		{ ...body, profiles: [{ ...body.profiles[0], evidence: { ...evidence, outcome: 'failed' } }] }
	])('rejects unsafe or inconsistent responses', async (value) => {
		const { client } = clientFor(value);

		await expect(client.getRepositoryReadiness('octo', 'demo')).rejects.toMatchObject({
			status: 502,
			message: 'The Agents service returned an invalid response.'
		});
	});

	it('preserves missing catalog recovery', async () => {
		const value = {
			...body,
			catalog_state: 'missing',
			profiles: [],
			diagnostics: [{ code: 'MISSING_FILE', path: '/', message: 'Add the catalog.' }]
		};
		const { client } = clientFor(value);
		expect(await client.getRepositoryReadiness('octo', 'demo')).toEqual(value);
	});

	it.each([403, 404, 429, 502])('returns safe recovery for HTTP %s', async (status) => {
		const { client } = clientFor({ message: 'private-sentinel' }, status);

		await expect(client.getRepositoryReadiness('octo', 'demo')).rejects.toMatchObject({ status });
		if (status === 403) {
			await expect(client.getRepositoryReadiness('octo', 'demo')).rejects.toThrow('Actions');
		}
	});
});
