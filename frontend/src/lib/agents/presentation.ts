import type { AgentProfile, AgentWorkspace, ProfileEvidence, ReadinessState } from './types';

export const viewMaxAge = 5 * 60 * 1000;
const evidenceMaxAge = 24 * 60 * 60 * 1000;

export const readinessLabels: Record<ReadinessState, string> = {
	configuration_missing: 'Configuration missing',
	verification_pending: 'Awaiting runtime verification',
	runtime_verified: 'Runtime verified',
	verification_failed: 'Verification failed'
};

const reasonLabels: Record<ProfileEvidence['reason'], string> = {
	verified: 'Live diagnostic passed',
	installation_unavailable: 'Codex installation could not be verified',
	authentication_unavailable: 'Runner sign-in needs attention',
	request_failed: 'The live diagnostic request failed',
	effective_policy_unavailable: 'Exact model and reasoning policy could not be verified'
};

export function evidenceReason(reason: string): string {
	return reasonLabels[reason] ?? 'Inspect the safe diagnostic result on GitHub';
}

export function workspaceIsStale(workspace: AgentWorkspace, now: number): boolean {
	if (workspace.state === 'error') {
		return false;
	}
	if (workspace.inconsistent || now - Date.parse(workspace.loadedAt) >= viewMaxAge) {
		return true;
	}
	return workspace.profiles.some(
		({ readiness }) =>
			readiness?.evidence && now - Date.parse(readiness.evidence.verifiedAt) > evidenceMaxAge
	);
}

export function runtimeLabel(profile: AgentProfile, stale: boolean): string {
	if (stale) {
		return 'Refresh to verify';
	}
	if (!profile.readiness) {
		return 'Readiness unavailable';
	}
	if (profile.readiness.state === 'configuration_missing') {
		return 'Not verified';
	}
	return readinessLabels[profile.readiness.state];
}

export function setupLabel(profile: AgentProfile, stale: boolean): string {
	if (stale || !profile.readiness) {
		return 'Refresh to check setup';
	}
	return profile.readiness.state === 'configuration_missing'
		? 'Required files or workflows missing'
		: 'Required configuration present';
}

export function formatTimestamp(timestamp: string): string {
	return (
		new Intl.DateTimeFormat('en-GB', {
			dateStyle: 'medium',
			timeStyle: 'short',
			timeZone: 'UTC'
		}).format(new Date(timestamp)) + ' UTC'
	);
}

export function profilesQuery(repository: string): string {
	return '?' + new URLSearchParams({ repository }).toString();
}
