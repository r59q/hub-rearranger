import { describe, expect, it, vi } from 'vitest';
import { env } from '$env/dynamic/private';
import { AgentsApiClient, AgentsApiError, agentsClient } from './agents-api';
import type { AssignmentConvention } from './agents-response';

const convention = {
	version: 1,
	profile_catalog_path: '.github/agent-profiles.yml',
	profile_schema_version: 1,
	assignment: {
		event: 'issue_comment.created',
		source_kind: 'issue',
		command_template: '/agent assign {profile_id}@{profile_revision} authority=branch-draft-pr',
		authority: 'branch-draft-pr',
		required_roles: ['maintain', 'admin'],
		profile_revision: { format: 'full-40-character-sha', default_branch_ancestor: true }
	}
} satisfies AssignmentConvention;

describe('AgentsApiClient', () => {
	it('reads the convention from the private service with no credentials', async () => {
		const request = vi.fn<typeof fetch>().mockResolvedValue(Response.json(convention));
		const client = new AgentsApiClient('http://agents:8082', request);

		const result = await client.getAssignmentConvention();

		expect(result).toEqual(convention);
		const outgoing = request.mock.calls[0][0] as Request;
		expect(outgoing.url).toBe('http://agents:8082/v1/assignment-convention');
		expect(outgoing.method).toBe('GET');
		expect(outgoing.headers.get('accept')).toBe('application/json');
		expect(outgoing.headers.has('authorization')).toBe(false);
	});

	it('uses the server-only configured address', async () => {
		const previous = env.AGENTS_API_URL;
		env.AGENTS_API_URL = 'http://private-agents:9090';
		const request = vi.fn<typeof fetch>().mockResolvedValue(Response.json(convention));

		try {
			await agentsClient(request).getAssignmentConvention();

			expect((request.mock.calls[0][0] as Request).url).toBe(
				'http://private-agents:9090/v1/assignment-convention'
			);
		} finally {
			if (previous === undefined) {
				delete env.AGENTS_API_URL;
			} else {
				env.AGENTS_API_URL = previous;
			}
		}
	});

	it('uses loopback when no service address is configured', async () => {
		const previous = env.AGENTS_API_URL;
		delete env.AGENTS_API_URL;
		const request = vi.fn<typeof fetch>().mockResolvedValue(Response.json(convention));

		try {
			await agentsClient(request).getAssignmentConvention();

			expect((request.mock.calls[0][0] as Request).url).toBe(
				'http://127.0.0.1:8082/v1/assignment-convention'
			);
		} finally {
			if (previous !== undefined) {
				env.AGENTS_API_URL = previous;
			}
		}
	});

	it('keeps upstream failure details out of user-facing errors', async () => {
		const request = vi
			.fn<typeof fetch>()
			.mockResolvedValue(
				Response.json(
					{ code: 'internal_error', message: 'private upstream detail' },
					{ status: 500 }
				)
			);
		const client = new AgentsApiClient('http://agents:8082', request);

		await expect(client.getAssignmentConvention()).rejects.toMatchObject({
			name: 'AgentsApiError',
			status: 500,
			message: 'The Agents service could not complete the request. Try again.'
		});
	});

	it.each([
		{},
		{ ...convention, version: 2 },
		{ ...convention, assignment: null },
		{ ...convention, assignment: { ...convention.assignment, authority: 'merge' } },
		{ ...convention, assignment: { ...convention.assignment, required_roles: ['admin', 'admin'] } },
		{ ...convention, assignment: { ...convention.assignment, required_roles: ['write', 'admin'] } },
		{
			...convention,
			assignment: {
				...convention.assignment,
				profile_revision: { format: 'branch', default_branch_ancestor: false }
			}
		},
		{ ...convention, unexpected: 'private upstream detail' }
	])('rejects malformed or unsupported policy responses: %j', async (body) => {
		const request = vi.fn<typeof fetch>().mockResolvedValue(Response.json(body));
		const client = new AgentsApiClient('http://agents:8082', request);

		await expect(client.getAssignmentConvention()).rejects.toMatchObject({
			status: 502,
			message: 'The Agents service returned an invalid response.'
		});
	});

	it('reports invalid JSON safely', async () => {
		const request = vi
			.fn<typeof fetch>()
			.mockResolvedValue(
				new Response('private-sentinel {', { headers: { 'content-type': 'application/json' } })
			);
		const client = new AgentsApiClient('http://agents:8082', request);

		await expect(client.getAssignmentConvention()).rejects.toMatchObject({ status: 502 });
	});

	it('reports an unavailable service without affecting other clients', async () => {
		const request = vi.fn<typeof fetch>().mockRejectedValue(new Error('private network detail'));
		const client = new AgentsApiClient('http://agents:8082', request);

		await expect(client.getAssignmentConvention()).rejects.toEqual(
			expect.objectContaining<Partial<AgentsApiError>>({
				status: 503,
				message: 'The Agents service is unavailable. Start it and try again.'
			})
		);
	});
});
