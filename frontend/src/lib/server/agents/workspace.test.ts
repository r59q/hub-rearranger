import { describe, expect, it, vi } from 'vitest';
import { AgentsApiError } from '../agents-api';
import { loadAgentWorkspace } from './workspace';
import { catalogFixture, readinessFixture, loadedAt, revision } from '../../../test/agent-fixtures';

function reader() {
	return {
		getRepositoryProfiles: vi.fn().mockResolvedValue(catalogFixture()),
		getRepositoryReadiness: vi.fn().mockResolvedValue(readinessFixture())
	};
}

const now = () => new Date(loadedAt);

describe('agent workspace projection', () => {
	it('joins matching policy and readiness, keeping source links pinned', async () => {
		const source = reader();

		const result = await loadAgentWorkspace(source, 'octo', 'demo', now);

		expect(result).toMatchObject({
			state: 'ready',
			revision,
			loadedAt: now().toISOString(),
			readinessError: null,
			inconsistent: false
		});
		expect(result.profiles[0]).toMatchObject({
			id: 'codex-thorough',
			name: 'Codex Thorough',
			role: 'implementation',
			authority: 'branch-draft-pr',
			runner: 'hub-agent-codex',
			model: 'gpt-6.1-sol',
			effort: 'high',
			readiness: {
				state: 'runtime_verified',
				evidence: { runUrl: 'https://github.com/octo/demo/actions/runs/10/attempts/2' }
			}
		});
		expect(result.catalogUrl).toBe(
			`https://github.com/octo/demo/blob/${revision}/.github/agent-profiles.yml`
		);
		expect(result.commitUrl).toBe(`https://github.com/octo/demo/commit/${revision}`);
		expect(source.getRepositoryProfiles).toHaveBeenCalledWith('octo', 'demo');
	});

	it.each(['missing', 'invalid', 'unsupported'] as const)(
		'preserves %s catalog recovery without partial profiles',
		async (state) => {
			const source = reader();
			source.getRepositoryProfiles.mockResolvedValue({
				...catalogFixture(),
				state,
				profiles: [],
				diagnostics: [
					{
						code:
							state === 'missing'
								? 'MISSING_FILE'
								: state === 'unsupported'
									? 'UNSUPPORTED_VERSION'
									: 'INVALID_YAML',
						path: '/',
						message: 'Fix the catalog.'
					}
				]
			});

			const result = await loadAgentWorkspace(source, 'octo', 'demo', now);

			expect(result.state).toBe(`catalog_${state}`);
			expect(result.profiles).toEqual([]);
			expect(result.diagnostics[0].message).toBe('Fix the catalog.');
			if (state === 'missing') {
				expect(result.catalogUrl).toBe(`https://github.com/octo/demo/tree/${revision}`);
			}
		}
	);

	it('keeps policy visible when Actions reads are unavailable', async () => {
		const source = reader();
		source.getRepositoryReadiness.mockRejectedValue(
			new AgentsApiError('Grant Actions read access.', 403)
		);

		const result = await loadAgentWorkspace(source, 'octo', 'demo', now);

		expect(result).toMatchObject({ state: 'ready', readinessError: 'Grant Actions read access.' });
		expect(result.profiles[0].readiness).toBeNull();
	});

	it('does not expose orphan readiness or raw exceptions when profiles fail', async () => {
		const source = reader();
		source.getRepositoryProfiles.mockRejectedValue(new Error('private-sentinel'));

		const result = await loadAgentWorkspace(source, 'octo', 'demo', now);

		expect(result).toMatchObject({
			state: 'error',
			error: 'Repository profiles could not be loaded. Try again.',
			profiles: []
		});
		expect(JSON.stringify(result)).not.toContain('private-sentinel');
	});

	it.each([
		{ ...readinessFixture(), revision: 'f'.repeat(40) },
		{ ...readinessFixture(), default_branch: 'trunk' },
		{ ...readinessFixture(), repository: 'other/repo' },
		{ ...readinessFixture(), catalog_state: 'missing' },
		{ ...readinessFixture(), profiles: [] },
		{ ...readinessFixture(), profiles: [{ ...readinessFixture().profiles[0], id: 'other' }] },
		{
			...readinessFixture(),
			profiles: [{ ...readinessFixture().profiles[0], runner_label: 'addons' }]
		},
		{
			...readinessFixture(),
			profiles: [
				{
					...readinessFixture().profiles[0],
					evidence: { ...readinessFixture().profiles[0].evidence!, requested_model: 'other' }
				}
			]
		}
	])('drops verification when snapshots or policies disagree', async (readiness) => {
		const source = reader();
		source.getRepositoryReadiness.mockResolvedValue(readiness);

		const result = await loadAgentWorkspace(source, 'octo', 'demo', now);

		expect(result.inconsistent).toBe(true);
		expect(result.readinessError).toContain('Refresh');
		expect(result.profiles[0].readiness).toBeNull();
	});

	it('retains disabled profiles without changing their configured policy', async () => {
		const source = reader();
		const catalog = catalogFixture();
		catalog.profiles[0].configuration.enabled = false;
		source.getRepositoryProfiles.mockResolvedValue(catalog);

		const result = await loadAgentWorkspace(source, 'octo', 'demo', now);

		expect(result.profiles[0]).toMatchObject({
			enabled: false,
			authority: 'branch-draft-pr',
			runner: 'hub-agent-codex'
		});
	});
});
