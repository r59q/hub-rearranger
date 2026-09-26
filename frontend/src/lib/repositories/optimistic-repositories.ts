import type { Repository } from './types';

export function mergeOptimisticRepositories(
	persisted: Repository[],
	optimistic: Repository[]
): Repository[] {
	const persistedIds = new Set(persisted.map(({ id }) => id));
	return [...persisted, ...optimistic.filter(({ id }) => !persistedIds.has(id))];
}
