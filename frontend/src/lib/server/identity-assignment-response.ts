import type { components } from './identity-contract.gen';

type Review = components['schemas']['AssignmentReview'];
type Result = components['schemas']['AssignmentResult'];

export function isAssignmentReview(
	value: unknown,
	repository: string,
	number: number,
	revision: string
): value is Review {
	if (!value || typeof value !== 'object') {
		return false;
	}
	const row = value as Review;
	return (
		Object.keys(row).length === 8 &&
		Object.keys(row).every((key) =>
			[
				'review_token',
				'repository',
				'number',
				'title',
				'profile_revision',
				'command',
				'user',
				'expires_at'
			].includes(key)
		) &&
		row.repository === repository &&
		row.number === number &&
		row.profile_revision === revision &&
		row.command === `/agent assign codex-thorough@${revision} authority=branch-draft-pr` &&
		typeof row.review_token === 'string' &&
		/^[A-Za-z0-9_-]{43}$/.test(row.review_token) &&
		typeof row.title === 'string' &&
		row.title.length <= 1024 &&
		typeof row.expires_at === 'string' &&
		Number.isFinite(Date.parse(row.expires_at)) &&
		!!row.user &&
		Object.keys(row.user).length === 2 &&
		Number.isSafeInteger(row.user.id) &&
		row.user.id > 0 &&
		typeof row.user.login === 'string' &&
		/^[A-Za-z0-9][A-Za-z0-9-]{0,38}$/.test(row.user.login)
	);
}

export function isAssignmentResult(
	value: unknown,
	repository: string,
	number: number
): value is Result {
	if (!value || typeof value !== 'object') {
		return false;
	}
	const row = value as Result;
	return (
		Object.keys(row).length === 4 &&
		Object.keys(row).every((key) =>
			['repository', 'number', 'comment_id', 'comment_url'].includes(key)
		) &&
		row.repository === repository &&
		row.number === number &&
		Number.isSafeInteger(row.comment_id) &&
		row.comment_id > 0 &&
		row.comment_url ===
			`https://github.com/${repository}/issues/${number}#issuecomment-${row.comment_id}`
	);
}
