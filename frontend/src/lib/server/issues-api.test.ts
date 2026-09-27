import { describe, expect, it, vi } from 'vitest';
import { IssuesApiClient, IssuesApiError } from './issues-api';

describe('IssuesApiClient', () => {
	it('loads a page for every selected repository', async () => {
		const request = vi.fn<typeof fetch>().mockResolvedValue(
			new Response(
				JSON.stringify({
					issues: [],
					page: 2,
					per_page: 6,
					has_next: true,
					unavailable_repositories: [],
					incomplete_details: 0
				}),
				{ status: 200, headers: { 'content-type': 'application/json' } }
			)
		);
		const client = new IssuesApiClient('http://issues:8081', request);

		const page = await client.listRecent(['octo/alpha', 'octo/beta'], 2);

		expect(page.has_next).toBe(true);
		const requestedURL = request.mock.calls[0][0] as URL;
		expect(requestedURL.pathname).toBe('/v1/issues');
		expect(requestedURL.searchParams.getAll('repository')).toEqual(['octo/alpha', 'octo/beta']);
		expect(requestedURL.searchParams.get('page')).toBe('2');
	});

	it('exposes safe service errors', async () => {
		const request = vi.fn<typeof fetch>().mockResolvedValue(
			new Response(JSON.stringify({ message: 'GitHub issues are temporarily unavailable.' }), {
				status: 502
			})
		);
		const client = new IssuesApiClient('http://issues:8081', request);

		await expect(client.listRecent(['octo/demo'])).rejects.toEqual(
			expect.objectContaining<Partial<IssuesApiError>>({
				message: 'GitHub issues are temporarily unavailable.',
				status: 502
			})
		);
	});
});
