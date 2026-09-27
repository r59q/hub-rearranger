import type { Repository } from './types';

export function filterRepositories(repositories: Repository[], query: string): Repository[] {
	const normalizedQuery = query.trim().toLocaleLowerCase();
	if (!normalizedQuery) {
		return repositories;
	}

	return repositories.filter(
		({ name, full_name }) =>
			name.toLocaleLowerCase().includes(normalizedQuery) ||
			full_name.toLocaleLowerCase().includes(normalizedQuery)
	);
}
