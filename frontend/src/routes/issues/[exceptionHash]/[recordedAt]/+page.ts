export const prerender = false;

export function load({ params }) {
    return {
        exceptionHash: params.exceptionHash,
        recordedAt: decodeURIComponent(params.recordedAt)
    };
}
