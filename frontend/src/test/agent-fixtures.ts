import type { Repository } from '$lib/repositories/types';
import type { RepositoryProfiles } from '$lib/server/agents-profiles-response';
import type { RepositoryReadiness } from '$lib/server/agents-readiness-response';
import type { ReadinessState } from '$lib/agents/types';

export const revision = '0123456789abcdef0123456789abcdef01234567';
export const loadedAt = '2026-10-01T12:00:00Z';
export const repository: Repository = {
	id: 1,
	owner: 'octo',
	name: 'demo',
	full_name: 'octo/demo',
	html_url: 'https://github.com/octo/demo',
	description: 'Synthetic public repository',
	private: false,
	default_branch: 'main',
	selected: true
};

export function catalogFixture(): RepositoryProfiles {
	return {
		repository: repository.full_name,
		catalog_path: '.github/agent-profiles.yml',
		default_branch: 'main',
		revision,
		state: 'valid',
		diagnostics: [],
		profiles: [
			{
				id: 'codex-thorough',
				revision,
				configuration: {
					enabled: true,
					name: 'Codex Thorough',
					description: 'Implement an issue and address trusted review feedback.',
					role: 'implementation',
					adapter: {
						id: 'codex-chatgpt-private-runner',
						contract_version: 1,
						runner_label: 'hub-agent-codex'
					},
					model: { id: 'gpt-6.1-sol', reasoning_effort: 'high', fallback: 'none' },
					triggers: { assignment: 'issue_comment.created' },
					context: {
						sources: ['issue', 'repository', 'pull_request', 'review_thread', 'checks'],
						images: false
					},
					authority: { mode: 'branch-draft-pr', sandbox: 'workspace-write', network: false },
					validation: { checks: ['repository-check'], on_failure: 'draft-with-evidence' },
					continuation: { review_comments: true, pipeline: 'disabled' }
				}
			}
		]
	};
}

export function readinessFixture(state: ReadinessState = 'runtime_verified'): RepositoryReadiness {
	return {
		repository: repository.full_name,
		default_branch: 'main',
		revision,
		catalog_state: 'valid',
		diagnostics: [],
		profiles: [
			{
				id: 'codex-thorough',
				revision,
				runner_label: 'hub-agent-codex',
				state,
				next_action:
					state === 'runtime_verified'
						? 'Refresh after changes or within 24 hours.'
						: 'Complete setup, then run the profile diagnostic.',
				diagnostics:
					state === 'runtime_verified'
						? []
						: [
								{
									code:
										state === 'configuration_missing'
											? 'MISSING_FILE'
											: state === 'verification_failed'
												? 'RUNTIME_FAILED'
												: 'EVIDENCE_MISSING',
									path:
										state === 'configuration_missing'
											? '.github/workflows/agent-assignment.yml'
											: '.github/workflows/agent-profile-diagnostic.yml',
									message:
										state === 'configuration_missing'
											? 'Add this required repository file.'
											: 'Run the profile diagnostic.'
								}
							],
				...(state === 'runtime_verified' || state === 'verification_failed'
					? {
							evidence: {
								version: 1,
								repository: repository.full_name,
								profile_id: 'codex-thorough',
								profile_revision: revision,
								runner_label: 'hub-agent-codex',
								requested_model: 'gpt-6.1-sol',
								requested_reasoning_effort: 'high',
								effective_model: state === 'runtime_verified' ? 'gpt-6.1-sol' : null,
								effective_reasoning_effort: state === 'runtime_verified' ? 'high' : null,
								cli_version: 'codex-cli 0.157.1',
								run_id: 10,
								run_attempt: 2,
								verified_at: loadedAt,
								outcome: state === 'runtime_verified' ? 'verified' : 'failed',
								reason_code: state === 'runtime_verified' ? 'verified' : 'request_failed'
							}
						}
					: {})
			}
		]
	};
}
