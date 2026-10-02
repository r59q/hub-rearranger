import { resolve } from '$app/paths';
import {
	identityFor,
	identityFailure,
	publicRedirect,
	sameOrigin
} from '$lib/server/identity-http';
import { IdentityApiError } from '$lib/server/identity-api';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async (event) => {
	if (!sameOrigin(event)) {
		return new Response('The request could not be verified.', { status: 403 });
	}

	try {
		const form = await event.request.formData();
		const csrf = form.get('csrf');
		if (typeof csrf !== 'string' || form.getAll('csrf').length !== 1 || csrf.length !== 43) {
			throw new IdentityApiError(403);
		}

		const result = await identityFor(event).signOut(csrf);
		return publicRedirect(
			resolve('/account') +
				'?' +
				new URLSearchParams({ notice: result.revoked ? 'signed-out' : 'revocation-unconfirmed' }),
			result.response.headers.getSetCookie()
		);
	} catch (error) {
		return identityFailure(error);
	}
};
