import { describe, expect, it, vi } from 'vitest';
import { load } from './+page.server';
import type { PageServerLoadEvent } from './$types';

describe('account page identity', () => {
	it('clears an invalid opaque cookie while keeping safe recovery state', async () => {
		const input = {
			url: new URL('https://hub.example.com'),
			request: new Request('https://hub.example.com', {
				headers: { cookie: '__Host-hub_session=' + 's'.repeat(43) }
			}),
			fetch: vi
				.fn<typeof fetch>()
				.mockResolvedValue(Response.json({ message: 'private-sentinel' }, { status: 401 })),
			cookies: { delete: vi.fn() }
		} as unknown as PageServerLoadEvent;

		const result = await load(input);

		expect(result).toEqual({
			identity: { state: 'reconnect_required', user: null, csrf: null, expiresAt: null }
		});
		expect(input.cookies.delete).toHaveBeenCalledWith('__Host-hub_session', { path: '/' });
	});
});
