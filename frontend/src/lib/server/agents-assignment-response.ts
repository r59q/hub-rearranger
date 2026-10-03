import type { components } from './agents-contract.gen';

export type IssueAssignment = components['schemas']['IssueAssignment'];
const sha = /^[0-9a-f]{40}$/;
const states = [
	'requested',
	'queued',
	'running',
	'blocked',
	'failed',
	'cancelled',
	'completed',
	'proposal',
	'unavailable'
];
const id = (value: unknown): value is number => Number.isSafeInteger(value) && Number(value) > 0;
const text = (value: unknown, maximum = 2000): value is string =>
	typeof value === 'string' && value.length <= maximum;

function closed(value: unknown, keys: string[]): value is Record<string, unknown> {
	return (
		!!value &&
		typeof value === 'object' &&
		!Array.isArray(value) &&
		Object.keys(value).length === keys.length &&
		keys.every((key) => Object.hasOwn(value, key))
	);
}

export function isIssueAssignment(
	value: unknown,
	repository: string,
	number: number
): value is IssueAssignment {
	if (
		!closed(value, [
			'repository',
			'repository_id',
			'number',
			'title',
			'body',
			'url',
			'issue_state',
			'profile_revision',
			'assignable',
			'reason',
			'command',
			'last_comment_id',
			'requests'
		])
	) {
		return false;
	}
	const source = `https://github.com/${repository}/issues/${number}`;
	if (
		value.repository !== repository ||
		value.number !== number ||
		!id(value.repository_id) ||
		!text(value.title, 1024) ||
		!text(value.body, 8000) ||
		value.url !== source ||
		!['open', 'closed'].includes(String(value.issue_state)) ||
		!text(value.profile_revision, 40) ||
		!sha.test(value.profile_revision) ||
		typeof value.assignable !== 'boolean' ||
		!text(value.reason) ||
		value.command !==
			`/agent assign codex-thorough@${value.profile_revision} authority=branch-draft-pr` ||
		!Number.isSafeInteger(value.last_comment_id) ||
		Number(value.last_comment_id) < 0 ||
		!Array.isArray(value.requests) ||
		value.requests.length > 10
	) {
		return false;
	}
	const seen = new Set<number>();
	for (const request of value.requests) {
		if (
			!closed(request, [
				'comment_id',
				'url',
				'requester_id',
				'requester',
				'profile_revision',
				'created_at',
				'edited',
				'state',
				'run',
				'proposal',
				'summary'
			]) ||
			!id(request.comment_id) ||
			seen.has(request.comment_id) ||
			request.comment_id > Number(value.last_comment_id) ||
			request.url !== `${source}#issuecomment-${request.comment_id}` ||
			!id(request.requester_id) ||
			!text(request.requester, 39) ||
			!/^[A-Za-z0-9][A-Za-z0-9-]{0,38}$/.test(request.requester) ||
			!text(request.profile_revision, 40) ||
			!sha.test(request.profile_revision) ||
			!text(request.created_at, 40) ||
			!Number.isFinite(Date.parse(request.created_at)) ||
			typeof request.edited !== 'boolean' ||
			!states.includes(String(request.state)) ||
			!text(request.summary)
		) {
			return false;
		}
		seen.add(request.comment_id);
		const run = request.run;
		if (
			run !== null &&
			(!closed(run, ['id', 'attempt', 'url', 'status', 'conclusion']) ||
				!id(run.id) ||
				!id(run.attempt) ||
				run.url !== `https://github.com/${repository}/actions/runs/${run.id}` ||
				!text(run.status, 50) ||
				!text(run.conclusion, 50))
		) {
			return false;
		}
		const proposal = request.proposal;
		if (proposal !== null) {
			const branch = `agent/codex-thorough/${value.repository_id}-${request.comment_id}`;
			if (
				!run ||
				!closed(proposal, [
					'number',
					'url',
					'state',
					'draft',
					'branch',
					'branch_url',
					'head_sha',
					'check'
				]) ||
				!id(proposal.number) ||
				proposal.url !== `https://github.com/${repository}/pull/${proposal.number}` ||
				!['open', 'closed'].includes(String(proposal.state)) ||
				typeof proposal.draft !== 'boolean' ||
				proposal.branch !== branch ||
				proposal.branch_url !== `https://github.com/${repository}/tree/${branch}` ||
				!text(proposal.head_sha, 40) ||
				!sha.test(proposal.head_sha)
			) {
				return false;
			}
			const check = proposal.check;
			if (
				check !== null &&
				(!closed(check, ['url', 'status', 'conclusion', 'summary']) ||
					!text(check.url) ||
					!new RegExp(
						`^https://github\\.com/${repository.replaceAll('.', '\\.').replaceAll('-', '\\-')}/runs/[1-9][0-9]*$`
					).test(check.url) ||
					check.status !== 'completed' ||
					!['success', 'neutral'].includes(String(check.conclusion)) ||
					!text(check.summary))
			) {
				return false;
			}
		}
		if (request.state === 'proposal' && proposal === null) {
			return false;
		}
	}
	return !(value.assignable && (value.issue_state !== 'open' || value.requests.length > 0));
}
