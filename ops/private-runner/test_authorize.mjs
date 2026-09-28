import assert from 'node:assert/strict';
import test from 'node:test';
import { authorizeMaintainers } from './authorize.mjs';

function githubWithRoles(roles) {
	const calls = [];
	return {
		calls,
		rest: {
			repos: {
				async getCollaboratorPermissionLevel({ owner, repo, username }) {
					calls.push({ owner, repo, username });
					const role = roles[username];
					if (role instanceof Error) {
						throw role;
					}
					return { data: { role_name: role } };
				}
			}
		}
	};
}

test('allows a maintainer and admin, checking both rerun actors', async () => {
	const github = githubWithRoles({ original: 'maintain', rerunner: 'admin' });

	await authorizeMaintainers(github, 'owner', 'repo', ['original', 'rerunner']);

	assert.deepEqual(github.calls, [
		{ owner: 'owner', repo: 'repo', username: 'original' },
		{ owner: 'owner', repo: 'repo', username: 'rerunner' }
	]);
});

test('rejects write access even when its legacy permission looks like maintain', async () => {
	const github = githubWithRoles({ contributor: 'write' });

	await assert.rejects(
		authorizeMaintainers(github, 'owner', 'repo', ['contributor']),
		/Only repository maintainers and admins/
	);
});

test('rejects an untrusted rerun actor', async () => {
	const github = githubWithRoles({ original: 'admin', rerunner: 'read' });

	await assert.rejects(
		authorizeMaintainers(github, 'owner', 'repo', ['original', 'rerunner']),
		/Only repository maintainers and admins/
	);
});

test('fails closed when GitHub cannot verify a role', async () => {
	const github = githubWithRoles({ original: new Error('private API response') });

	await assert.rejects(
		authorizeMaintainers(github, 'owner', 'repo', ['original']),
		(error) => error.message.includes('Could not verify') && !error.message.includes('private API response')
	);
});

test('deduplicates original actor on a normal run', async () => {
	const github = githubWithRoles({ owner: 'admin' });

	await authorizeMaintainers(github, 'owner', 'repo', ['owner', 'owner']);

	assert.equal(github.calls.length, 1);
});
