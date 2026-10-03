import { fail, redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { issueAssignment } from '$lib/server/agents-assignment-api';
import { boundedForm, selectedRepository } from '$lib/server/agents-setup-http';
import { identityFor, sameOrigin } from '$lib/server/identity-http';
import { IdentityApiError } from '$lib/server/identity-api';
import type { Actions, PageServerLoad } from './$types';

function issueNumber(url: URL): number {
	const value = url.searchParams.get('number');
	const number = Number(value);
	return value &&
		/^[1-9][0-9]*$/.test(value) &&
		Number.isSafeInteger(number) &&
		number <= 2147483647
		? number
		: 0;
}

export const load: PageServerLoad = async (event) => {
	event.setHeaders({ 'cache-control': 'no-store' });
	const identity = identityFor(event).session();
	try {
		const repository = await selectedRepository(event);
		const number = issueNumber(event.url);
		if (!repository || !number) {
			return {
				identity: await identity,
				repository: null,
				assignment: null,
				error: 'Choose an issue from a selected workspace repository.'
			};
		}
		const assignment = await issueAssignment(
			event.fetch,
			repository.owner,
			repository.name,
			number
		);
		return { identity: await identity, repository, assignment, error: null };
	} catch {
		return {
			identity: await identity,
			repository: null,
			assignment: null,
			error:
				'Issue assignment context is unavailable. Check repository access and the Agents service, then refresh.'
		};
	}
};

async function submission(event: Parameters<NonNullable<Actions['review']>>[0], create: boolean) {
	if (!sameOrigin(event)) {
		return fail(403, {
			error: 'The request could not be verified. Refresh before trying again.',
			review: null,
			reconnect: false
		});
	}
	let result;
	try {
		if (
			!event.request.headers.get('content-type')?.startsWith('application/x-www-form-urlencoded')
		) {
			throw new IdentityApiError(400);
		}
		const form = new URLSearchParams(await boundedForm(event.request));
		const fields = create ? ['csrf', 'review_token', 'confirm'] : ['csrf', 'profile_revision'];
		if (
			[...form.keys()].some((key) => !fields.includes(key)) ||
			fields.some((key) => form.getAll(key).length !== 1) ||
			!/^[A-Za-z0-9_-]{43}$/.test(form.get('csrf') ?? '') ||
			(create
				? !/^[A-Za-z0-9_-]{43}$/.test(form.get('review_token') ?? '') ||
					form.get('confirm') !== 'reviewed'
				: !/^[0-9a-f]{40}$/.test(form.get('profile_revision') ?? ''))
		) {
			return fail(400, {
				error: 'Review the assignment authority and confirm the GitHub comment before submitting.',
				review: null,
				reconnect: false
			});
		}
		const repository = await selectedRepository(event);
		const number = issueNumber(event.url);
		if (!repository || !number) {
			return fail(400, {
				error: 'This issue repository is no longer selected. Return to Issues.',
				review: null,
				reconnect: false
			});
		}
		const identity = identityFor(event);
		if (!create) {
			const review = await identity.reviewAssignment(
				repository.owner,
				repository.name,
				number,
				form.get('csrf')!,
				form.get('profile_revision')!
			);
			return { error: null, review, reconnect: false };
		}
		result = await identity.createAssignment(
			repository.owner,
			repository.name,
			number,
			form.get('csrf')!,
			form.get('review_token')!
		);
	} catch (error) {
		const status = error instanceof IdentityApiError ? error.status : 503;
		return fail(status >= 400 && status < 600 ? status : 503, {
			error:
				error instanceof IdentityApiError
					? error.message
					: 'Assignment is unavailable. Inspect GitHub comments and refresh before making another request.',
			review: null,
			reconnect: status === 401
		});
	}
	// Browser refresh reads authoritative GitHub state instead of repeating a POST.
	redirect(
		303,
		resolve('/issues/assignment') +
			'?' +
			new URLSearchParams({ repository: result.repository, number: String(result.number) })
	);
}

export const actions: Actions = {
	review: (event) => submission(event, false),
	assign: (event) => submission(event, true)
};
