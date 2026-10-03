import type { IssueAssignment } from '../lib/server/agents-assignment-response';

export const revision = 'a'.repeat(40);
export const csrf = 'c'.repeat(43);
export const command = `/agent assign codex-thorough@${revision} authority=branch-draft-pr`;
export const review = {
	review_token: 't'.repeat(43),
	repository: 'octo/demo',
	number: 3,
	title: 'Implement a feature',
	profile_revision: revision,
	command,
	user: { id: 7, login: 'octocat' },
	expires_at: '2026-10-03T12:10:00Z'
};
export const assignmentResult = {
	repository: 'octo/demo',
	number: 3,
	comment_id: 99,
	comment_url: 'https://github.com/octo/demo/issues/3#issuecomment-99'
};

export function assignment(withProposal = false): IssueAssignment {
	return {
		repository: 'octo/demo',
		repository_id: 42,
		number: 3,
		title: 'Implement a feature',
		body: 'Issue context',
		url: 'https://github.com/octo/demo/issues/3',
		issue_state: 'open',
		profile_revision: revision,
		assignable: !withProposal,
		reason: 'Intake checks access and runner readiness.',
		command,
		last_comment_id: withProposal ? 99 : 10,
		requests: withProposal
			? [
					{
						comment_id: 99,
						url: assignmentResult.comment_url,
						requester_id: 7,
						requester: 'octocat',
						profile_revision: revision,
						created_at: '2026-10-03T12:00:00Z',
						edited: false,
						state: 'proposal',
						run: {
							id: 321,
							attempt: 1,
							url: 'https://github.com/octo/demo/actions/runs/321',
							status: 'completed',
							conclusion: 'success'
						},
						summary: 'Review the patch proposal and its checks.',
						proposal: {
							number: 5,
							url: 'https://github.com/octo/demo/pull/5',
							state: 'open',
							draft: true,
							branch: 'agent/codex-thorough/42-99',
							branch_url: 'https://github.com/octo/demo/tree/agent/codex-thorough/42-99',
							head_sha: 'b'.repeat(40),
							check: {
								url: 'https://github.com/octo/demo/runs/77',
								status: 'completed',
								conclusion: 'neutral',
								summary: 'Repository validation: unavailable. Human review is required.'
							}
						}
					}
				]
			: []
	};
}
