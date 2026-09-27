import type { Repository } from './types';

export interface RepositorySelectionGateway {
	listAvailable(): Promise<Repository[]>;
	replaceSelection(repositoryIds: number[]): Promise<Repository[]>;
}

export class RepositoryNotAvailableError extends Error {
	constructor() {
		super('This repository is no longer available to add. Refresh the page and try again.');
		this.name = 'RepositoryNotAvailableError';
	}
}

export async function addRepositoryToSelection(
	gateway: RepositorySelectionGateway,
	repositoryId: number
): Promise<void> {
	const repositories = await gateway.listAvailable();
	const repository = repositories.find(({ id }) => id === repositoryId);
	if (!repository) {
		throw new RepositoryNotAvailableError();
	}
	if (repository.selected) {
		return;
	}

	const selectedIds = repositories.filter(({ selected }) => selected).map(({ id }) => id);
	await gateway.replaceSelection([...selectedIds, repositoryId]);
}
