import { env } from '$env/dynamic/private';
import createClient from 'openapi-fetch';
import type { components, paths } from './identity-contract.gen';
import type { IdentityState } from '$lib/identity/types';

type Session = components['schemas']['Session'];

export class IdentityApiError extends Error {
	constructor(
		readonly status: number,
		code?: string
	) {
		super(
			status === 409
				? code === 'bootstrap_incomplete'
					? 'Publication could not be confirmed. Reload and review again; Hub will reconcile the existing GitHub branch and PR before retrying.'
					: 'The reviewed base or bootstrap branch changed. Reload and review the current changes before trying again.'
				: status === 401
					? 'Your GitHub connection expired or was revoked. Sign in again.'
					: status === 403
						? 'The request could not be verified or repository access is unavailable. Reload and check your GitHub access.'
						: 'GitHub sign-in is unavailable. Try again.'
		);
	}
}

function isSession(value: unknown): value is Session {
	if (!value || typeof value !== 'object') {
		return false;
	}

	const row = value as Record<string, unknown>;
	if (Object.keys(row).some((key) => !['state', 'user', 'csrf', 'expires_at'].includes(key))) {
		return false;
	}

	if (row.state === 'signed_out' || row.state === 'disabled') {
		return row.user === null && row.csrf === null && row.expires_at === null;
	}
	if (row.state !== 'authenticated' || !row.user || typeof row.user !== 'object') {
		return false;
	}

	const user = row.user as Record<string, unknown>;
	return (
		Object.keys(user).every((key) => ['id', 'login'].includes(key)) &&
		Number.isSafeInteger(user.id) &&
		Number(user.id) > 0 &&
		typeof user.login === 'string' &&
		/^[a-zA-Z0-9][a-zA-Z0-9-]{0,38}$/.test(user.login) &&
		typeof row.csrf === 'string' &&
		/^[A-Za-z0-9_-]{43}$/.test(row.csrf) &&
		typeof row.expires_at === 'string' &&
		Number.isFinite(Date.parse(row.expires_at))
	);
}

export class IdentityApiClient {
	private readonly api;

	constructor(
		private readonly baseUrl: string,
		private readonly request: typeof fetch,
		private readonly cookie: string,
		private readonly origin: string
	) {
		this.api = createClient<paths>({
			baseUrl,
			fetch: request,
			headers: { cookie, Origin: origin },
			redirect: 'manual',
			credentials: 'omit'
		});
	}

	async session(): Promise<IdentityState> {
		try {
			const { data, response } = await this.api.GET('/v1/session', {
				signal: AbortSignal.timeout(25_000)
			});
			if (response.status === 401) {
				return { state: 'reconnect_required', user: null, csrf: null, expiresAt: null };
			}
			if (!response.ok || !isSession(data)) {
				throw new IdentityApiError(response.ok ? 502 : response.status);
			}

			return { state: data.state, user: data.user, csrf: data.csrf, expiresAt: data.expires_at };
		} catch {
			return { state: 'unavailable', user: null, csrf: null, expiresAt: null };
		}
	}

	async createBootstrap(
		owner: string,
		repo: string,
		csrf: string,
		baseRevision: string,
		digest: string,
		profileDraft?: components['schemas']['BootstrapRequest']['profile_draft']
	) {
		try {
			const { data, error, response } = await this.api.POST(
				'/v1/repositories/{owner}/{repo}/bootstrap-pull-request',
				{
					params: { path: { owner, repo }, header: { Origin: this.origin } },
					body: {
						csrf,
						base_revision: baseRevision,
						digest,
						...(profileDraft ? { profile_draft: profileDraft } : {})
					},
					signal: AbortSignal.timeout(125_000)
				}
			);
			if (!response.ok) {
				throw new IdentityApiError(response.status, error?.code);
			}
			if (
				!data ||
				Object.keys(data).some(
					(key) =>
						![
							'repository',
							'branch',
							'head_sha',
							'pull_request_number',
							'pull_request_url'
						].includes(key)
				) ||
				data.repository !== `${owner}/${repo}` ||
				!data.branch.endsWith(`-${baseRevision.slice(0, 12)}-${digest}`) ||
				!/^hub-bootstrap\/codex-thorough-[0-9]+-[0-9a-f]{12}-[0-9a-f]{64}$/.test(data.branch) ||
				!/^[0-9a-f]{40}$/.test(data.head_sha) ||
				!Number.isSafeInteger(data.pull_request_number) ||
				data.pull_request_number <= 0 ||
				data.pull_request_url !==
					`https://github.com/${owner}/${repo}/pull/${data.pull_request_number}`
			) {
				throw new IdentityApiError(502);
			}
			return data;
		} catch (error) {
			if (error instanceof IdentityApiError) {
				throw error;
			}
			throw new IdentityApiError(503);
		}
	}

	async start(): Promise<Response> {
		return this.redirect('/v1/sign-in', { method: 'POST' });
	}

	async complete(code: string, state: string): Promise<Response> {
		return this.redirect('/v1/sign-in/callback?' + new URLSearchParams({ code, state }), {
			method: 'GET'
		});
	}

	private async redirect(path: string, options: RequestInit): Promise<Response> {
		try {
			const response = await this.request(this.baseUrl + path, {
				...options,
				headers: { cookie: this.cookie, Origin: this.origin },
				redirect: 'manual',
				credentials: 'omit',
				signal: AbortSignal.timeout(25_000)
			});
			if (response.status !== 303) {
				throw new IdentityApiError(response.status);
			}

			const location = new URL(response.headers.get('location') ?? '');
			const expected =
				path === '/v1/sign-in'
					? location.origin === 'https://github.com' &&
						location.pathname === '/login/oauth/authorize' &&
						!location.username &&
						!location.password
					: location.href === this.origin + '/account';
			if (!expected) {
				throw new IdentityApiError(502);
			}

			return response;
		} catch (error) {
			if (error instanceof IdentityApiError) {
				throw error;
			}
			throw new IdentityApiError(503);
		}
	}

	async signOut(csrf: string): Promise<{ revoked: boolean; message: string; response: Response }> {
		try {
			const { data, response } = await this.api.POST('/v1/sign-out', {
				params: { header: { Origin: this.origin } },
				body: { csrf },
				signal: AbortSignal.timeout(25_000)
			});
			if (!response.ok || !data || typeof data.revoked !== 'boolean') {
				throw new IdentityApiError(response.ok ? 502 : response.status);
			}

			// Copy only fixed recovery text; never relay provider/service prose to the UI.
			return {
				revoked: data.revoked,
				message: data.revoked
					? 'Signed out. Your GitHub token was revoked.'
					: 'Signed out of Hub. Revoke the App authorization in GitHub account settings to finish disconnecting.',
				response
			};
		} catch (error) {
			if (error instanceof IdentityApiError) {
				throw error;
			}
			throw new IdentityApiError(503);
		}
	}
}

export function identityClient(
	request: typeof fetch,
	cookie: string,
	origin: string
): IdentityApiClient {
	return new IdentityApiClient(
		env.IDENTITY_API_URL ?? 'http://127.0.0.1:8083',
		request,
		cookie,
		origin
	);
}
