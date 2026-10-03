import { env } from '$env/dynamic/private';
import createClient from 'openapi-fetch';
import { createHash } from 'node:crypto';
import type { components, paths } from './agents-contract.gen';

type Preview = components['schemas']['BootstrapPreview'];

export type BootstrapView = Pick<
	Preview,
	| 'repository'
	| 'default_branch'
	| 'base_revision'
	| 'digest'
	| 'state'
	| 'private'
	| 'diagnostics'
	| 'diff'
> & {
	files: Pick<Preview['files'][number], 'path' | 'status'>[];
};

const sha = /^[0-9a-f]{40}$/;
const digest = /^[0-9a-f]{64}$/;
const keys = (row: Record<string, unknown>, allowed: string[]) =>
	Object.keys(row).every((key) => allowed.includes(key));

function validPreview(value: unknown, repository: string): value is Preview {
	if (!value || typeof value !== 'object') {
		return false;
	}
	const row = value as Record<string, unknown>;
	if (
		!keys(row, [
			'repository',
			'default_branch',
			'base_revision',
			'digest',
			'state',
			'private',
			'files',
			'diagnostics',
			'diff'
		]) ||
		row.repository !== repository ||
		typeof row.default_branch !== 'string' ||
		!row.default_branch ||
		typeof row.base_revision !== 'string' ||
		!sha.test(row.base_revision) ||
		typeof row.digest !== 'string' ||
		!digest.test(row.digest) ||
		!['ready', 'unchanged', 'conflict'].includes(String(row.state)) ||
		typeof row.private !== 'boolean' ||
		typeof row.diff !== 'string' ||
		row.diff.length > 8 << 20 ||
		!Array.isArray(row.files) ||
		row.files.length === 0 ||
		row.files.length > 256 ||
		!Array.isArray(row.diagnostics)
	) {
		return false;
	}
	const seen = new Set<string>();
	for (const file of row.files) {
		if (
			!file ||
			typeof file !== 'object' ||
			!keys(file, ['path', 'status', 'base_sha', 'sha256', 'content']) ||
			typeof file.path !== 'string' ||
			!file.path ||
			file.path.startsWith('/') ||
			file.path.split('/').some((part: string) => !part || part === '.' || part === '..') ||
			file.path.includes('\\') ||
			seen.has(file.path) ||
			!['create', 'update', 'unchanged', 'conflict'].includes(file.status) ||
			typeof file.content !== 'string' ||
			Buffer.byteLength(file.content) > 1 << 20 ||
			(file.base_sha !== null && (typeof file.base_sha !== 'string' || !sha.test(file.base_sha)))
		) {
			return false;
		}
		if (
			file.status !== 'conflict' &&
			(typeof file.sha256 !== 'string' ||
				createHash('sha256').update(file.content).digest('hex') !== file.sha256)
		) {
			return false;
		}
		if (
			(file.status === 'create' && file.base_sha !== null) ||
			(['update', 'unchanged'].includes(file.status) && file.base_sha === null)
		) {
			return false;
		}
		seen.add(file.path);
	}
	if (
		!row.diagnostics.every(
			(item) =>
				item &&
				typeof item === 'object' &&
				keys(item, ['code', 'path', 'message']) &&
				['code', 'path', 'message'].every((key) => typeof item[key] === 'string')
		)
	) {
		return false;
	}
	return row.state === 'conflict'
		? row.diagnostics.length > 0
		: row.diagnostics.length === 0 &&
				row.files.every((file) => file.status !== 'conflict') &&
				(row.state === 'ready'
					? row.files.some((file) => ['create', 'update'].includes(file.status))
					: row.files.every((file) => file.status === 'unchanged'));
}

export async function bootstrapPreview(
	request: typeof fetch,
	owner: string,
	repo: string
): Promise<BootstrapView> {
	const api = createClient<paths>({
		baseUrl: env.AGENTS_API_URL ?? 'http://127.0.0.1:8082',
		fetch: request,
		redirect: 'manual',
		credentials: 'omit'
	});
	try {
		const { data, response } = await api.GET('/v1/repositories/{owner}/{repo}/bootstrap', {
			params: { path: { owner, repo } },
			signal: AbortSignal.timeout(50_000)
		});
		if (!response.ok || !validPreview(data, `${owner}/${repo}`)) {
			throw new Error();
		}
		// Source bytes are needed only by Identity's fresh server-side plan. The
		// browser receives the review diff and filenames, never a write payload.
		return bootstrapView(data, `${owner}/${repo}`);
	} catch {
		throw new Error(
			'Bootstrap changes could not be read. Check repository access and the Agents service, then reload.'
		);
	}
}

export function bootstrapView(value: unknown, repository: string): BootstrapView {
	if (!validPreview(value, repository)) {
		throw new Error('The Agents service returned an invalid review. Reload before continuing.');
	}
	return { ...value, files: value.files.map(({ path, status }) => ({ path, status })) };
}
