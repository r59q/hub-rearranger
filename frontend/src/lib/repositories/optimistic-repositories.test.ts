import { describe, expect, it } from 'vitest';
import { mergeOptimisticRepositories } from './optimistic-repositories';
import type { Repository } from './types';

const persisted = [{ id: 1, name: 'alpha' }] as Repository[];
const optimistic = [{ id: 2, name: 'beta' }] as Repository[];

describe('mergeOptimisticRepositories', () => {
	it('includes a pending repository immediately', () => {
		expect(mergeOptimisticRepositories(persisted, optimistic)).toEqual([
			persisted[0],
			optimistic[0]
		]);
	});

	it('does not duplicate a repository after the server confirms it', () => {
		const confirmed = [{ ...optimistic[0], selected: true }] as Repository[];

		expect(mergeOptimisticRepositories(confirmed, optimistic)).toEqual(confirmed);
	});
});
