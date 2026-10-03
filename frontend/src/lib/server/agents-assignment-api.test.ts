import { describe, expect, it, vi } from 'vitest';
import { issueAssignment } from './agents-assignment-api';
import { assignment } from '../../test/assignment-fixtures';

describe('assignment GitHub projection adapter', () => {
	it('reads with discovery access and exposes canonical source links', async () => {
		const request = vi.fn<typeof fetch>().mockResolvedValue(Response.json(assignment(true)));
		expect(await issueAssignment(request, 'octo', 'demo', 3)).toEqual(assignment(true));
		const outgoing = request.mock.calls[0][0] as Request;
		expect(outgoing.method).toBe('GET');
		expect(outgoing.credentials).toBe('omit');
		expect(outgoing.headers.has('authorization')).toBe(false);
		expect(outgoing.headers.has('cookie')).toBe(false);
	});
	it.each([
		'repository',
		'command',
		'secret',
		'run-url',
		'pr-url',
		'branch',
		'check',
		'duplicate',
		'assignable',
		'large',
		'missing'
	])('rejects malformed %s projection', async (variant) => {
		const data = assignment(true);
		switch (variant) {
			case 'repository':
				data.repository = 'foreign/repo';
				break;
			case 'command':
				data.command = '/agent assign forged';
				break;
			case 'secret':
				Object.assign(data, { access_token: 'synthetic-never-live' });
				break;
			case 'run-url':
				data.requests[0].run!.url = 'https://foreign.example';
				break;
			case 'pr-url':
				data.requests[0].proposal!.url = 'javascript:alert(1)';
				break;
			case 'branch':
				data.requests[0].proposal!.branch = 'main';
				break;
			case 'check':
				data.requests[0].proposal!.check!.url = 'https://github.com/foreign/repo/runs/77';
				break;
			case 'duplicate':
				data.requests.push(data.requests[0]);
				break;
			case 'assignable':
				data.assignable = true;
				break;
			case 'large':
				data.body = 'x'.repeat(8001);
				break;
			case 'missing':
				Reflect.deleteProperty(data.requests[0], 'run');
				break;
		}
		await expect(
			issueAssignment(
				vi.fn<typeof fetch>().mockResolvedValue(Response.json(data)),
				'octo',
				'demo',
				3
			)
		).rejects.toThrow('context is unavailable');
	});
});
