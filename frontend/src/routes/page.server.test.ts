import { describe, expect, it, vi } from 'vitest';
import { load } from './+page.server';

const selectedRepository = {
	id: 1,
	owner: 'octo',
	name: 'demo',
	full_name: 'octo/demo',
	html_url: 'https://github.com/octo/demo',
	description: '',
	private: false,
	default_branch: 'main',
	selected: true
};

const availableRepository = { ...selectedRepository, id: 2, name: 'other', selected: false };

describe('home page load', () => {
	it('loads the requested issue page for selected repositories', async () => {
		const request = vi.fn<typeof fetch>().mockImplementation(async (input) => {
			const url = new URL(input instanceof Request ? input.url : input.toString());
			if (url.pathname === '/v1/repositories/available') {
				return Response.json({ repositories: [selectedRepository, availableRepository] });
			}
			if (url.pathname === '/v1/issues') {
				expect(url.searchParams.getAll('repository')).toEqual(['octo/demo']);
				expect(url.searchParams.get('page')).toBe('2');
				return Response.json({
					issues: [],
					page: 2,
					per_page: 6,
					has_next: false,
					unavailable_repositories: [],
					incomplete_details: 0
				});
			}
			return new Response(null, { status: 404 });
		});

		const result = await load({
			fetch: request,
			url: new URL('http://localhost/?issues=open&issuePage=2')
		} as never);

		expect(result).toEqual(
			expect.objectContaining({
				repositories: [selectedRepository],
				issueViewExpanded: true,
				issueError: null,
				issuePage: expect.objectContaining({ page: 2 })
			})
		);
	});

	it('keeps repositories usable when the issues service fails', async () => {
		const request = vi.fn<typeof fetch>().mockImplementation(async (input) => {
			const url = new URL(input instanceof Request ? input.url : input.toString());
			if (url.pathname === '/v1/repositories/available') {
				return Response.json({ repositories: [selectedRepository] });
			}
			return Response.json({ message: 'Issues are rate limited.' }, { status: 502 });
		});

		const result = await load({ fetch: request, url: new URL('http://localhost/') } as never);

		expect(result).toEqual(
			expect.objectContaining({
				repositories: [selectedRepository],
				serviceError: null,
				issueError: 'Issues are rate limited.'
			})
		);
	});
});
