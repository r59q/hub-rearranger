import { describe, expect, it } from 'vitest';
import {
	workspaceIsStale,
	viewMaxAge,
	runtimeLabel,
	setupLabel,
	profilesQuery,
	evidenceReason
} from './presentation';
import { loadAgentWorkspace } from '$lib/server/agents/workspace';
import { catalogFixture, readinessFixture, loadedAt } from '../../test/agent-fixtures';

async function workspace() {
	return loadAgentWorkspace(
		{
			getRepositoryProfiles: async () => catalogFixture(),

			getRepositoryReadiness: async () => readinessFixture()
		},
		'octo',
		'demo',
		() => new Date(loadedAt)
	);
}

describe('agent view freshness and labels', () => {
	it('marks five-minute snapshots stale and suppresses a verified badge', async () => {
		const view = await workspace();
		const now = Date.parse(loadedAt);
		expect(workspaceIsStale(view, now + viewMaxAge - 1)).toBe(false);

		expect(workspaceIsStale(view, now + viewMaxAge)).toBe(true);

		expect(runtimeLabel(view.profiles[0], true)).toBe('Refresh to verify');

		expect(runtimeLabel(view.profiles[0], false)).toBe('Runtime verified');
	});

	it('marks expiring evidence stale even when the view was just loaded', async () => {
		const view = await workspace();
		const now = Date.parse(loadedAt);
		view.profiles[0].readiness!.evidence!.verifiedAt = new Date(
			now - 24 * 60 * 60 * 1000
		).toISOString();
		expect(workspaceIsStale(view, now)).toBe(false);

		expect(workspaceIsStale(view, now + 1)).toBe(true);
	});

	it('marks inconsistent and missing-catalog snapshots stale', async () => {
		const view = await workspace();
		view.inconsistent = true;
		expect(workspaceIsStale(view, Date.parse(loadedAt))).toBe(true);
		view.inconsistent = false;
		view.state = 'catalog_missing';
		view.profiles = [];
		expect(workspaceIsStale(view, Date.parse(loadedAt) + viewMaxAge)).toBe(true);
	});

	it('distinguishes missing setup, pending, failed, and unavailable runtime', async () => {
		const view = await workspace();
		const profile = view.profiles[0];
		profile.readiness!.state = 'configuration_missing';
		expect(setupLabel(profile, false)).toBe('Required files or workflows missing');

		expect(runtimeLabel(profile, false)).toBe('Not verified');
		profile.readiness!.state = 'verification_pending';
		expect(runtimeLabel(profile, false)).toBe('Awaiting runtime verification');
		profile.readiness!.state = 'verification_failed';
		expect(runtimeLabel(profile, false)).toBe('Verification failed');
		profile.readiness = null;
		expect(runtimeLabel(profile, false)).toBe('Readiness unavailable');
	});

	it('encodes repository navigation and uses safe outcome explanations', () => {
		expect(new URLSearchParams(profilesQuery('octo/demo').slice(1)).get('repository')).toBe(
			'octo/demo'
		);
		expect(evidenceReason('authentication_unavailable')).toBe('Runner sign-in needs attention');
		expect(evidenceReason('private-sentinel')).not.toContain('private-sentinel');
	});
});
