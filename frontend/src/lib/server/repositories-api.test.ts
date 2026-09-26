import { describe, expect, it, vi } from 'vitest';
import { RepositoriesApiClient, RepositoriesApiError } from './repositories-api';

describe('RepositoriesApiClient', () => {
	it('loads available repositories from the configured service', async () => {
		const request = vi.fn<typeof fetch>().mockResolvedValue(
			new Response(
				JSON.stringify({
					repositories: [
						{
							id: 1,
							owner: 'octo',
							name: 'demo',
							full_name: 'octo/demo',
							html_url: 'https://github.com/octo/demo',
							description: '',
							private: false,
							default_branch: 'main',
							selected: false
						}
					]
				}),
				{ status: 200, headers: { 'content-type': 'application/json' } }
			)
		);
		const client = new RepositoriesApiClient('http://repositories:8080', request);

		const repositories = await client.listAvailable();

		expect(repositories[0].full_name).toBe('octo/demo');
		expect(request).toHaveBeenCalledWith(
			new URL('http://repositories:8080/v1/repositories/available'),
			expect.objectContaining({ headers: { accept: 'application/json' } })
		);
	});

	it('passes selected IDs and exposes safe API errors', async () => {
		const request = vi.fn<typeof fetch>().mockResolvedValue(
			new Response(JSON.stringify({ message: 'Selection is no longer available.' }), {
				status: 422
			})
		);
		const client = new RepositoriesApiClient('http://repositories:8080', request);

		const result = client.replaceSelection([2, 4]);

		await expect(result).rejects.toEqual(
			expect.objectContaining<Partial<RepositoriesApiError>>({
				message: 'Selection is no longer available.',
				status: 422
			})
		);
		expect(request).toHaveBeenCalledWith(
			new URL('http://repositories:8080/v1/repository-selection'),
			expect.objectContaining({ body: JSON.stringify({ repository_ids: [2, 4] }) })
		);
	});
});
