export type EditorDraft = {
	name: string;
	description: string;
	enabled: boolean;
	contextSources: string[];
	reviewComments: boolean;
};

export const contextChoices = [
	{ id: 'issue', label: 'Source issue', required: true },
	{ id: 'repository', label: 'Repository files', required: true },
	{ id: 'issue_comments', label: 'Issue conversation', required: false },
	{ id: 'instructions', label: 'Repository instructions', required: false },
	{ id: 'pull_request', label: 'Pull request', required: false },
	{ id: 'review_thread', label: 'Review thread', required: false },
	{ id: 'checks', label: 'Related checks', required: false }
];

// This checks browser draft storage and form shape. Agents remains responsible
// for canonical schema validation and adapter policy, including dependencies.
export function isEditorDraft(value: unknown): value is EditorDraft {
	if (!value || typeof value !== 'object') {
		return false;
	}
	const row = value as Record<string, unknown>;
	return (
		Object.keys(row).length === 5 &&
		typeof row.name === 'string' &&
		row.name.length <= 80 &&
		typeof row.description === 'string' &&
		row.description.length <= 500 &&
		typeof row.enabled === 'boolean' &&
		typeof row.reviewComments === 'boolean' &&
		Array.isArray(row.contextSources) &&
		row.contextSources.length <= 7 &&
		new Set(row.contextSources).size === row.contextSources.length &&
		row.contextSources.every((source) => contextChoices.some(({ id }) => id === source))
	);
}
