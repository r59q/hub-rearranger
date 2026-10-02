import { describe, expect, it, vi } from 'vitest';
import { load } from './+page.server';
import { repository, catalogFixture, readinessFixture } from '../../test/agent-fixtures';

function requestFor(
	options: { selected?: boolean; readinessStatus?: number; repositoryStatus?: number } = {}
) {
	return vi.fn<typeof fetch>().mockImplementation(async (input) => {
		const url = new URL(input instanceof Request ? input.url : input.toString());
		if (url.pathname === '/v1/repositories') {
			return Response.json(
				{ repositories: options.selected === false ? [] : [repository] },
				{ status: options.repositoryStatus ?? 200 }
			);
		}
		if (url.pathname.endsWith('/profiles')) {
			return Response.json(catalogFixture());
		}
		if (url.pathname.endsWith('/readiness')) {
			return Response.json(
				options.readinessStatus ? { message: 'private-sentinel' } : readinessFixture(),
				{ status: options.readinessStatus ?? 200 }
			);
		}
		return new Response(null, { status: 404 });
	});
}

function event(fetch: typeof globalThis.fetch, query = '', isDataRequest = false) {
	return {
		fetch,
		url: new URL('http://localhost/agents' + query),
		isDataRequest,
		setHeaders: vi.fn()
	};
}

describe('Agents page server load', () => {
	it('loads the selected repository through server-only adapters with no-store', async () => {
		const request = requestFor();
		const input = event(request, '?repository=octo%2Fdemo');

		const result = await load(input as never);

		expect(result).toMatchObject({
			repository,
			serviceError: null,
			workspace: { state: 'ready', repository: 'octo/demo' }
		});
		expect(input.setHeaders).toHaveBeenCalledWith({ 'cache-control': 'no-store' });
		const outgoing = request.mock.calls.map(([input]) =>
			input instanceof Request ? input.url : input.toString()
		);
		expect(outgoing.some((url) => url.endsWith('/v1/repositories/octo/demo/profiles'))).toBe(true);
		expect(outgoing.some((url) => url.endsWith('/v1/repositories/octo/demo/readiness'))).toBe(true);
	});

	it('defaults to the first selected repository', async () => {
		const result = await load(event(requestFor()) as never);

		expect(result?.repository).toEqual(repository);
	});

	it('streams caught results on client navigation while initial HTML is complete', async () => {
		const input = event(requestFor(), '?repository=octo%2Fdemo', true);

		const result = await load(input as never);

		expect(result?.workspace).toBeInstanceOf(Promise);
		await expect(result?.workspace).resolves.toMatchObject({ state: 'ready' });
	});

	it('does not query Agents for an arbitrary repository outside the selected workspace', async () => {
		const request = requestFor();

		const result = await load(event(request, '?repository=other%2Frepo') as never);

		expect(result).toMatchObject({
			workspace: null,
			selectionError: 'Choose a repository currently selected in your workspace.'
		});
		expect(request).toHaveBeenCalledTimes(1);
	});

	it('returns an explicit empty selection without Agents reads', async () => {
		const request = requestFor({ selected: false });

		const result = await load(event(request) as never);

		expect(result).toMatchObject({
			repositories: [],
			repository: null,
			workspace: null,
			serviceError: null
		});
		expect(request).toHaveBeenCalledTimes(1);
	});

	it('keeps profile policy available during Actions permission failures', async () => {
		const result = await load(event(requestFor({ readinessStatus: 403 })) as never);
		const workspace = await result?.workspace;
		expect(workspace?.readinessError).toContain('Actions');
		expect(workspace?.profiles[0].readiness).toBeNull();
		expect(JSON.stringify(result)).not.toContain('private-sentinel');
	});

	it('keeps repository-service errors separate from an empty workspace', async () => {
		const result = await load(event(requestFor({ repositoryStatus: 503 })) as never);

		expect(result?.serviceError).toBeTruthy();
		expect(result?.workspace).toBeNull();
	});
});
