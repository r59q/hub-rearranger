import { createHash } from 'node:crypto';
import type { components } from '$lib/server/agents-contract.gen';
import type { IdentityState } from '$lib/identity/types';

export const base = 'a'.repeat(40);
export const digest = 'd'.repeat(64);
export const identity: IdentityState = {
	state: 'authenticated',
	user: { id: 7, login: 'octocat' },
	csrf: 'c'.repeat(43),
	expiresAt: '2026-10-08T12:00:00Z'
};
export const publication = {
	repository: 'octo/demo',
	branch: `hub-bootstrap/codex-thorough-7-${base.slice(0, 12)}-${digest}`,
	head_sha: 'b'.repeat(40),
	pull_request_number: 3,
	pull_request_url: 'https://github.com/octo/demo/pull/3'
};

export function preview(): components['schemas']['BootstrapPreview'] {
	return {
		repository: 'octo/demo',
		default_branch: 'main',
		base_revision: base,
		digest,
		state: 'ready',
		private: true,
		diagnostics: [],
		diff: '+ <script>alert(1)</script>\n',
		files: [
			{
				path: 'AGENTS.md',
				status: 'create',
				base_sha: null,
				sha256: createHash('sha256').update('setup').digest('hex'),
				content: 'setup'
			}
		]
	};
}
