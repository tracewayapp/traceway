import type { PageLoad } from './$types';

export const prerender = false;

export const load: PageLoad = ({ params }) => {
	return { token: params.token };
};
