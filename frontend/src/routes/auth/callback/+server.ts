import { identityFor, identityFailure, publicRedirect } from '$lib/server/identity-http';
import { IdentityApiError } from '$lib/server/identity-api';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async (event) => {
	try {
		const query = event.url.searchParams;
		if (
			query.getAll('code').length !== 1 ||
			query.getAll('state').length !== 1 ||
			query.has('error')
		) {
			throw new IdentityApiError(403);
		}

		const response = await identityFor(event).complete(query.get('code')!, query.get('state')!);
		return publicRedirect(response.headers.get('location')!, response.headers.getSetCookie());
	} catch (error) {
		return identityFailure(error);
	}
};
