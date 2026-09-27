import { describe, expect, it } from 'vitest';
import { formatIssueActivity, issuePageHref } from './presentation';

describe('issue presentation', () => {
	it('formats activity dates consistently in UTC', () => {
		expect(formatIssueActivity('2026-09-27T23:30:00-04:00')).toBe('Updated 28 Sept 2026');
	});

	it('builds an issue page link', () => {
		expect(issuePageHref(3)).toBe('?issuePage=3');
	});
});
