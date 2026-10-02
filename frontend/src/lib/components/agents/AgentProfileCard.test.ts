import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import AgentProfileCard from './AgentProfileCard.svelte';
import AgentWorkspaceView from './AgentWorkspaceView.svelte';
import { loadAgentWorkspace } from '$lib/server/agents/workspace';
import { catalogFixture, readinessFixture, loadedAt } from '../../../test/agent-fixtures';

async function profile(state: Parameters<typeof readinessFixture>[0] = 'runtime_verified') {
	const workspace = await loadAgentWorkspace(
		{
			getRepositoryProfiles: async () => catalogFixture(),

			getRepositoryReadiness: async () => readinessFixture(state)
		},
		'octo',
		'demo',
		() => new Date(loadedAt)
	);
	return workspace.profiles[0];
}

describe('agent profile card rendering', () => {
	it('suppresses current verification after a failed refresh', async () => {
		const workspace = await loadAgentWorkspace(
			{
				getRepositoryProfiles: async () => catalogFixture(),

				getRepositoryReadiness: async () => readinessFixture()
			},
			'octo',
			'demo',
			() => new Date(loadedAt)
		);

		const { body } = render(AgentWorkspaceView, { props: { workspace, refreshFailed: true } });

		expect(body).toContain('Refresh to verify');
		expect(body).not.toContain('Runtime verified');
	});

	it('renders decision context with semantic headings and evidence links', async () => {
		const value = await profile();

		const { body } = render(AgentProfileCard, { props: { profile: value } });
		for (const text of [
			'Codex Thorough',
			'codex-thorough',
			'Implementation',
			'Branch and draft pull request',
			'hub-agent-codex',
			'gpt-6.1-sol',
			'high reasoning',
			'Runtime verified',
			'Diagnostic evidence'
		]) {
			expect(body).toContain(text);
		}
		expect(body).toContain('aria-labelledby="profile-codex-thorough"');
		expect(body).toMatch(/<dl(?:\s[^>]*)?>/);
		expect(body).toContain('https://github.com/octo/demo/actions/runs/10/attempts/2');
	});

	it('shows missing files and an explicit next step', async () => {
		const value = await profile('configuration_missing');

		const { body } = render(AgentProfileCard, { props: { profile: value } });

		expect(body).toContain('Configuration missing');
		expect(body).toContain('.github/workflows/agent-assignment.yml');
		expect(body).toContain('Not verified');
		expect(body).toContain('Next step');
	});

	it('never presents stale evidence as current verification', async () => {
		const value = await profile();

		const { body } = render(AgentProfileCard, { props: { profile: value, stale: true } });

		expect(body).toContain('Results need refreshing');
		expect(body).toContain('Refresh to verify');
		expect(body).toContain('Previous diagnostic evidence');
		expect(body).not.toContain('Runtime verified');
	});

	it('escapes repository-owned prose and preserves disabled profiles', async () => {
		const value = await profile();
		value.name = '<script>private-sentinel</script>';
		value.description = '<img src=x onerror=alert(1)>';
		value.enabled = false;

		const { body } = render(AgentProfileCard, { props: { profile: value } });

		expect(body).not.toContain('<script>private-sentinel');
		expect(body).not.toContain('<img src=x');
		expect(body).toContain('This profile is disabled');
	});
});
