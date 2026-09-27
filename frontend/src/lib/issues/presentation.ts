export function formatIssueActivity(value: string): string {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) {
		return 'Activity time unavailable';
	}
	return `Updated ${new Intl.DateTimeFormat('en-GB', {
		day: 'numeric',
		month: 'short',
		year: 'numeric',
		timeZone: 'UTC'
	}).format(date)}`;
}

export function issuePageHref(page: number): string {
	const parameters = new URLSearchParams({ issues: 'open', issuePage: String(page) });
	return `?${parameters.toString()}`;
}
