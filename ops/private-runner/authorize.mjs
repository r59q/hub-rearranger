/** Check live GitHub roles before sending a job to the credentialed runner. */
export async function authorizeMaintainers(github, owner, repo, actors) {
	for (const username of new Set(actors)) {
		if (!/^[a-z\d](?:[a-z\d-]{0,37}[a-z\d])?$/i.test(username)) {
			throw new Error('The workflow actor could not be verified.');
		}

		let role;
		try {
			const response = await github.rest.repos.getCollaboratorPermissionLevel({
				owner,
				repo,
				username
			});
			role = response.data.role_name;
		} catch {
			throw new Error('Could not verify repository maintainer permission. Retry or inspect Actions access.');
		}

		// `permission: write` also includes the distinct `maintain` role.
		if (role !== 'maintain' && role !== 'admin') {
			throw new Error('Only repository maintainers and admins may run this diagnostic.');
		}
	}
}
