export const prerender = false;

export function load({ params, url }) {
    return {
        exceptionHash: params.exceptionHash,
        exceptionId: params.exceptionId,
        recordedAt: url.searchParams.get('t')
    };
}
