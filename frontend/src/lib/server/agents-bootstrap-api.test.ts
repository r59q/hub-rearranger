import { describe, expect, it, vi } from 'vitest';
import { bootstrapPreview } from './agents-bootstrap-api';
import { preview } from '../../test/bootstrap-fixtures';

describe('bootstrap read adapter', () => {
	it('validates hashes and exposes only display data', async () => {
		const request = vi.fn<typeof fetch>().mockResolvedValue(Response.json(preview()));
		const result = await bootstrapPreview(request, 'octo', 'demo');
		expect(result.files).toEqual([{ path: 'AGENTS.md', status: 'create' }]);
		expect(JSON.stringify(result)).not.toContain('"content"');
		const outgoing = request.mock.calls[0][0] as Request;
		expect(outgoing.credentials).toBe('omit');
		expect(outgoing.headers.has('cookie')).toBe(false);
		expect(outgoing.headers.has('authorization')).toBe(false);
	});
	it.each(['hash', 'status', 'path', 'duplicate', 'diagnostic', 'secret', 'repository', 'large'])(
		'rejects malformed %s fields',
		async (variant) => {
			const data = preview();
			switch (variant) {
				case 'hash':
					data.files[0].content = 'unbound';
					break;
				case 'status':
					data.state = 'unchanged';
					break;
				case 'path':
					data.files[0].path = '../escape';
					break;
				case 'duplicate':
					data.files.push(data.files[0]);
					break;
				case 'diagnostic':
					data.diagnostics.push({ code: 'bad', path: 'AGENTS.md', message: 'private-sentinel' });
					break;
				case 'secret':
					Object.assign(data, { access_token: 'synthetic-never-live' });
					break;
				case 'repository':
					data.repository = 'other/repo';
					break;
				case 'large':
					data.files[0].content = 'a'.repeat((1 << 20) + 1);
					break;
			}
			await expect(
				bootstrapPreview(
					vi.fn<typeof fetch>().mockResolvedValue(Response.json(data)),
					'octo',
					'demo'
				)
			).rejects.toThrow('Bootstrap changes could not be read');
		}
	);
	it('returns a conflict without inventing publishable bytes', async () => {
		const data = preview();
		data.state = 'conflict';
		data.files[0] = {
			path: 'AGENTS.md',
			status: 'conflict',
			base_sha: null,
			sha256: null,
			content: ''
		};
		data.diagnostics = [
			{ code: 'INSTRUCTIONS_CONFLICT', path: 'AGENTS.md', message: 'Review markers.' }
		];
		expect(
			(
				await bootstrapPreview(
					vi.fn<typeof fetch>().mockResolvedValue(Response.json(data)),
					'octo',
					'demo'
				)
			).state
		).toBe('conflict');
	});
});
