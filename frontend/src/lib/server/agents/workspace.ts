import type { RepositoryProfiles } from '../agents-profiles-response';
import type { RepositoryReadiness } from '../agents-readiness-response';
import { AgentsApiError } from '../agents-api';
import type { AgentProfile, AgentWorkspace } from '$lib/agents/types';

export interface AgentWorkspaceReader {
	getRepositoryProfiles(owner: string, repo: string): Promise<RepositoryProfiles>;
	getRepositoryReadiness(owner: string, repo: string): Promise<RepositoryReadiness>;
}

function repositoryUrl(owner: string, repo: string): string {
	return `https://github.com/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}`;
}

function readError(error: unknown, subject: string): string {
	return error instanceof AgentsApiError
		? error.message
		: `${subject} could not be loaded. Try again.`;
}

// The two reads can straddle a default-branch push. Join only identical catalog
// snapshots and profile sets; otherwise keep policy visible without runtime proof.
export async function loadAgentWorkspace(
	reader: AgentWorkspaceReader,
	owner: string,
	repo: string,
	now: () => Date = () => new Date()
): Promise<AgentWorkspace> {
	const baseUrl = repositoryUrl(owner, repo);
	const result: AgentWorkspace = {
		repository: `${owner}/${repo}`,
		branch: '',
		revision: '',
		catalogUrl: '',
		commitUrl: '',
		diagnosticUrl: `${baseUrl}/actions/workflows/agent-profile-diagnostic.yml`,
		loadedAt: '',
		state: 'error',
		profiles: [],
		diagnostics: [],
		error: null,
		readinessError: null,
		inconsistent: false
	};

	const [catalogResult, readinessResult] = await Promise.allSettled([
		reader.getRepositoryProfiles(owner, repo),
		reader.getRepositoryReadiness(owner, repo)
	]);
	result.loadedAt = now().toISOString();
	if (catalogResult.status === 'rejected') {
		result.error = readError(catalogResult.reason, 'Repository profiles');
		return result;
	}

	const catalog = catalogResult.value;
	result.branch = catalog.default_branch;
	result.revision = catalog.revision;
	result.catalogUrl = `${baseUrl}/blob/${catalog.revision}/.github/agent-profiles.yml`;
	result.commitUrl = `${baseUrl}/commit/${catalog.revision}`;
	result.diagnostics = catalog.diagnostics;
	if (catalog.state !== 'valid') {
		result.state = `catalog_${catalog.state}`;
		if (catalog.state === 'missing') {
			result.catalogUrl = `${baseUrl}/tree/${catalog.revision}`;
		}
		return result;
	}

	result.state = 'ready';
	let readiness: RepositoryReadiness | null = null;
	if (readinessResult.status === 'rejected') {
		result.readinessError = readError(readinessResult.reason, 'Runtime readiness');
	} else if (!sameSnapshot(catalog, readinessResult.value)) {
		result.inconsistent = true;
		result.readinessError =
			'The repository changed while profiles and readiness were loading. Refresh to read a matching revision.';
	} else {
		readiness = readinessResult.value;
	}

	result.profiles = catalog.profiles.map(({ id, configuration }) => {
		const row = readiness?.profiles.find((profile) => profile.id === id);
		const evidence = row?.evidence;

		const profile: AgentProfile = {
			id,
			name: configuration.name,
			description: configuration.description,
			enabled: configuration.enabled,
			role: configuration.role,
			model: configuration.model.id,
			effort: configuration.model.reasoning_effort,
			runner: configuration.adapter.runner_label,
			authority: configuration.authority.mode,
			sandbox: configuration.authority.sandbox,
			network: configuration.authority.network,
			checks: configuration.validation.checks,
			reviewComments: configuration.continuation.review_comments,
			readiness: row
				? {
						state: row.state,
						nextAction: row.next_action,
						diagnostics: row.diagnostics,
						evidence: evidence
							? {
									runUrl: `${baseUrl}/actions/runs/${evidence.run_id}/attempts/${evidence.run_attempt}`,
									verifiedAt: evidence.verified_at,
									cliVersion: evidence.cli_version,
									requestedModel: evidence.requested_model,
									requestedEffort: evidence.requested_reasoning_effort,
									effectiveModel: evidence.effective_model,
									effectiveEffort: evidence.effective_reasoning_effort,
									outcome: evidence.outcome,
									reason: evidence.reason_code
								}
							: null
					}
				: null
		};

		return profile;
	});

	return result;
}

function sameSnapshot(catalog: RepositoryProfiles, readiness: RepositoryReadiness): boolean {
	if (
		catalog.repository.toLowerCase() !== readiness.repository.toLowerCase() ||
		catalog.revision !== readiness.revision ||
		catalog.default_branch !== readiness.default_branch ||
		catalog.state !== readiness.catalog_state ||
		catalog.profiles.length !== readiness.profiles.length
	) {
		return false;
	}

	return catalog.profiles.every(({ id, revision, configuration }) => {
		const row = readiness.profiles.find((profile) => profile.id === id);
		if (
			!row ||
			row.revision !== revision ||
			row.runner_label !== configuration.adapter.runner_label
		) {
			return false;
		}

		return (
			!row.evidence ||
			(row.evidence.requested_model === configuration.model.id &&
				row.evidence.requested_reasoning_effort === configuration.model.reasoning_effort)
		);
	});
}
