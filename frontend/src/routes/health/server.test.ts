import { describe, expect, it } from 'vitest';
import { GET } from './+server';

describe('GET /health', () => {
	it('returns an OK status', async () => {
		const response = GET();

		expect(response.status).toBe(200);
		expect(await response.json()).toEqual({ status: 'ok' });
	});
});
