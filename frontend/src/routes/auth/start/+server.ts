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
		const response = await identityFor(event).start();
		return publicRedirect(response.headers.get('location')!, response.headers.getSetCookie());
	} catch (error) {
		return identityFailure(error);
	}
};

export const GET: RequestHandler = () => identityFailure(new IdentityApiError(403));
