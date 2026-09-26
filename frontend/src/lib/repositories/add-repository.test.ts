import { describe, expect, it, vi } from 'vitest';
import { addRepositoryToSelection, RepositoryNotAvailableError } from './add-repository';
import type { Repository } from './types';

function repository(id: number, selected = false): Repository {
	return {
		id,
		owner: 'octo',
		name: `repository-${id}`,
		full_name: `octo/repository-${id}`,
		html_url: `https://github.com/octo/repository-${id}`,
		description: '',
		private: false,
		default_branch: 'main',
		selected
	};
}

describe('addRepositoryToSelection', () => {
	it('preserves the current selection when adding a repository', async () => {
		const gateway = {
			listAvailable: vi.fn().mockResolvedValue([repository(1, true), repository(2)]),
			replaceSelection: vi.fn().mockResolvedValue([])
		};

		await addRepositoryToSelection(gateway, 2);

		expect(gateway.replaceSelection).toHaveBeenCalledWith([1, 2]);
	});

	it('does not rewrite an already selected repository', async () => {
		const gateway = {
			listAvailable: vi.fn().mockResolvedValue([repository(1, true)]),
			replaceSelection: vi.fn().mockResolvedValue([])
		};

		await addRepositoryToSelection(gateway, 1);

		expect(gateway.replaceSelection).not.toHaveBeenCalled();
	});

	it('rejects a repository that is not available from GitHub', async () => {
		const gateway = {
			listAvailable: vi.fn().mockResolvedValue([]),
			replaceSelection: vi.fn().mockResolvedValue([])
		};

		await expect(addRepositoryToSelection(gateway, 7)).rejects.toBeInstanceOf(
			RepositoryNotAvailableError
		);
	});
});
