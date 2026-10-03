import { describe, expect, it } from 'vitest';
import { isAssignmentReview, isAssignmentResult } from './identity-assignment-response';
import { assignmentResult, review, revision } from '../../test/assignment-fixtures';

describe('assignment write result boundary', () => {
	it('accepts only reviewed command and canonical comment result', () => {
		expect(isAssignmentReview(review, 'octo/demo', 3, revision)).toBe(true);
		expect(isAssignmentResult(assignmentResult, 'octo/demo', 3)).toBe(true);
	});
	it.each(['secret', 'command', 'user', 'repository', 'number', 'token', 'expiry'])(
		'rejects invalid %s review',
		(variant) => {
			const value = structuredClone(review);
			switch (variant) {
				case 'secret':
					Object.assign(value, { access_token: 'synthetic' });
					break;
				case 'command':
					value.command += ' extra';
					break;
				case 'user':
					value.user.id = 0;
					break;
				case 'repository':
					value.repository = 'other/repo';
					break;
				case 'number':
					value.number = 4;
					break;
				case 'token':
					value.review_token = 'forged';
					break;
				case 'expiry':
					value.expires_at = 'invalid';
					break;
			}
			expect(isAssignmentReview(value, 'octo/demo', 3, revision)).toBe(false);
		}
	);
	it('rejects forged comment links and secret fields', () => {
		expect(
			isAssignmentResult(
				{ ...assignmentResult, comment_url: 'https://foreign.example' },
				'octo/demo',
				3
			)
		).toBe(false);
		expect(isAssignmentResult({ ...assignmentResult, token: 'synthetic' }, 'octo/demo', 3)).toBe(
			false
		);
	});
});
