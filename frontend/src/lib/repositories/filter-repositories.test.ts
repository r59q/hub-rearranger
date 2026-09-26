import { describe, expect, it } from 'vitest';
import { filterRepositories } from './filter-repositories';
import type { Repository } from './types';

const repositories = [
	{ id: 1, owner: 'octo', name: 'Hub-Rearranger', full_name: 'octo/Hub-Rearranger' },
	{ id: 2, owner: 'agents', name: 'runner', full_name: 'agents/runner' }
] as Repository[];

describe('filterRepositories', () => {
	it('filters case-insensitively by repository name', () => {
		expect(filterRepositories(repositories, 'rearrange')).toEqual([repositories[0]]);
	});

	it('also supports a full owner and repository name', () => {
		expect(filterRepositories(repositories, 'AGENTS/RUN')).toEqual([repositories[1]]);
	});

	it('returns every repository for a blank query', () => {
		expect(filterRepositories(repositories, '   ')).toEqual(repositories);
	});
});
