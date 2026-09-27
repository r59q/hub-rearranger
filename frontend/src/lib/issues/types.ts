export type IssueLabel = {
	name: string;
	color: string;
};

export type RelatedIssue = {
	number: number;
	repository: string;
	title: string;
	state: string;
	html_url: string;
};

export type Issue = {
	id: number;
	number: number;
	repository: string;
	title: string;
	description: string;
	state: string;
	html_url: string;
	updated_at: string;
	labels: IssueLabel[];
	subtasks: RelatedIssue[];
	linked_issues: RelatedIssue[];
	linked_pull_requests: RelatedIssue[];
	details_available: boolean;
};

export type IssuePage = {
	issues: Issue[];
	page: number;
	per_page: number;
	has_next: boolean;
	unavailable_repositories: string[];
	incomplete_details: number;
};

export function emptyIssuePage(page = 1): IssuePage {
	return {
		issues: [],
		page,
		per_page: 6,
		has_next: false,
		unavailable_repositories: [],
		incomplete_details: 0
	};
}
