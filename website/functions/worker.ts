interface Env {
	ASSETS: { fetch(request: Request): Promise<Response> };
	SENDGRID_API_KEY: string;
	SENDGRID_FROM_EMAIL: string;
	SENDGRID_TO_EMAIL: string;
}

declare class FixedLengthStream extends TransformStream<Uint8Array, Uint8Array> {
	constructor(length: number);
}

const STREAM_PREFIX = "/stream";
const assetSizes = new Map<string, number>();

export default {
	async fetch(request: Request, env: Env): Promise<Response> {
		const url = new URL(request.url);

		if (
			url.pathname.startsWith(`${STREAM_PREFIX}/videos/`) &&
			(request.method === "GET" || request.method === "HEAD")
		) {
			return handleVideo(request, env, url);
		}

		if (url.pathname === "/api/contact" && request.method === "POST") {
			return handleContact(request, env);
		}

		if (url.pathname === "/api/subscribe" && request.method === "POST") {
			return handleSubscribe(request, env);
		}

		if (url.pathname === "/api/geo" && request.method === "GET") {
			return handleGeo(request);
		}

		return new Response("Not Found", { status: 404 });
	},
};

// Workers static assets ignore Range and answer 200 with the whole file
// (cloudflare/workers-sdk#8967). Chromium cannot seek in such a response and
// iOS Safari will not play it at all, so /stream/videos/* serves the assets
// under /videos/ with byte ranges.
async function handleVideo(request: Request, env: Env, url: URL): Promise<Response> {
	const assetUrl = new URL(url.pathname.slice(STREAM_PREFIX.length), url.origin);
	const asset = await env.ASSETS.fetch(new Request(assetUrl));

	// A missing file comes back as the SPA fallback page with a 200.
	const isVideo = (asset.headers.get("Content-Type") ?? "").startsWith("video/");
	if (!asset.ok || !asset.body || !isVideo) {
		await asset.body?.cancel();
		return new Response("Not Found", { status: 404 });
	}

	const etag = asset.headers.get("ETag");
	const headers = new Headers(asset.headers);
	headers.set("Accept-Ranges", "bytes");
	headers.delete("Content-Length");

	if (etag && request.headers.get("If-None-Match") === etag) {
		await asset.body.cancel();
		return new Response(null, { status: 304, headers });
	}

	// The assets binding does not report a length, and a range answer needs the
	// total, so the first request for a file in each isolate reads it whole.
	// Later ones stream. The ETag is a content hash, so the size never goes stale.
	const sizeKey = etag ?? assetUrl.pathname;
	let body = asset.body;
	let size = assetSizes.get(sizeKey);
	if (size === undefined) {
		const bytes = await asset.arrayBuffer();
		size = bytes.byteLength;
		assetSizes.set(sizeKey, size);
		body = new Response(bytes).body!;
	}

	const ifRange = request.headers.get("If-Range");
	const range =
		!ifRange || ifRange === etag ? parseRange(request.headers.get("Range"), size) : null;
	if (range === "unsatisfiable") {
		await body.cancel();
		headers.set("Content-Range", `bytes */${size}`);
		return new Response(null, { status: 416, headers });
	}

	const start = range ? range.start : 0;
	const length = (range ? range.end : size - 1) - start + 1;
	if (range) {
		headers.set("Content-Range", `bytes ${start}-${range.end}/${size}`);
	}
	const status = range ? 206 : 200;

	if (request.method === "HEAD") {
		await body.cancel();
		headers.set("Content-Length", String(length));
		return new Response(null, { status, headers });
	}

	const { readable, writable } = new FixedLengthStream(length);
	body
		.pipeThrough(byteSlice(start, length))
		.pipeTo(writable)
		.catch(() => {});
	return new Response(readable, { status, headers });
}

function parseRange(
	header: string | null,
	size: number
): { start: number; end: number } | "unsatisfiable" | null {
	const match = /^bytes=(\d*)-(\d*)$/.exec(header ?? "");
	if (!match || (!match[1] && !match[2])) return null;
	const [, first, last] = match;

	if (!first) {
		const suffix = Number(last);
		if (suffix === 0 || size === 0) return "unsatisfiable";
		return { start: Math.max(0, size - suffix), end: size - 1 };
	}

	const start = Number(first);
	if (last && Number(last) < start) return null;
	if (start >= size) return "unsatisfiable";
	return { start, end: last ? Math.min(Number(last), size - 1) : size - 1 };
}

function byteSlice(start: number, length: number): TransformStream<Uint8Array, Uint8Array> {
	let skip = start;
	let remaining = length;
	return new TransformStream({
		transform(chunk, controller) {
			if (skip >= chunk.byteLength) {
				skip -= chunk.byteLength;
				return;
			}
			const part = chunk.subarray(skip, skip + remaining);
			skip = 0;
			remaining -= part.byteLength;
			controller.enqueue(part);
			if (remaining === 0) controller.terminate();
		},
	});
}

async function handleContact(request: Request, env: Env): Promise<Response> {
	try {
		const formData = await request.json<any>();

		if (!formData.email || !formData.firstName || !formData.lastName) {
			return new Response(
				JSON.stringify({ error: "Missing required fields" }),
				{
					status: 400,
					headers: { "Content-Type": "application/json" },
				}
			);
		}

		const fullName = `${formData.firstName} ${formData.lastName}`.trim();
		const companySize = formData.companySize || "Not provided";

		const msg = {
			personalizations: [
				{
					to: [{ email: env.SENDGRID_TO_EMAIL }],
				},
			],
			from: { email: env.SENDGRID_FROM_EMAIL },
			reply_to: { email: formData.email, name: fullName },
			subject: `New signup from ${fullName} (${companySize})`,
			content: [
				{
					type: "text/plain",
					value: `
New signup from Traceway website:

Name: ${fullName}
Email: ${formData.email}
Company size: ${companySize}
					`.trim(),
				},
			],
		};

		const response = await fetch("https://api.sendgrid.com/v3/mail/send", {
			method: "POST",
			headers: {
				Authorization: `Bearer ${env.SENDGRID_API_KEY}`,
				"Content-Type": "application/json",
			},
			body: JSON.stringify(msg),
		});

		if (!response.ok) {
			const errorText = await response.text();
			console.error("SendGrid Error:", errorText);
			throw new Error(`SendGrid API error: ${response.status}`);
		}

		return new Response(JSON.stringify({ success: true }), {
			status: 200,
			headers: { "Content-Type": "application/json" },
		});
	} catch (error) {
		console.error("Error processing request:", error);
		return new Response(
			JSON.stringify({ error: "Internal Server Error" }),
			{
				status: 500,
				headers: { "Content-Type": "application/json" },
			}
		);
	}
}

async function handleSubscribe(request: Request, env: Env): Promise<Response> {
	try {
		const data = await request.json<any>();
		const email = typeof data.email === "string" ? data.email.trim() : "";

		if (!email || !/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email)) {
			return new Response(JSON.stringify({ error: "Invalid email" }), {
				status: 400,
				headers: { "Content-Type": "application/json" },
			});
		}

		const msg = {
			personalizations: [
				{
					to: [{ email: env.SENDGRID_TO_EMAIL }],
				},
			],
			from: { email: env.SENDGRID_FROM_EMAIL },
			reply_to: { email },
			subject: `New blog subscriber: ${email}`,
			content: [
				{
					type: "text/plain",
					value: `New blog subscriber from the Traceway Engineering blog:\n\nEmail: ${email}`,
				},
			],
		};

		const response = await fetch("https://api.sendgrid.com/v3/mail/send", {
			method: "POST",
			headers: {
				Authorization: `Bearer ${env.SENDGRID_API_KEY}`,
				"Content-Type": "application/json",
			},
			body: JSON.stringify(msg),
		});

		if (!response.ok) {
			const errorText = await response.text();
			console.error("SendGrid Error:", errorText);
			throw new Error(`SendGrid API error: ${response.status}`);
		}

		return new Response(JSON.stringify({ success: true }), {
			status: 200,
			headers: { "Content-Type": "application/json" },
		});
	} catch (error) {
		console.error("Error processing subscribe request:", error);
		return new Response(
			JSON.stringify({ error: "Internal Server Error" }),
			{
				status: 500,
				headers: { "Content-Type": "application/json" },
			}
		);
	}
}

// EEA, United Kingdom and Switzerland: the only places where analytics
// cookies need opt-in consent before they are set.
const CONSENT_REQUIRED_COUNTRIES = new Set([
	"AT", "BE", "BG", "HR", "CY", "CZ", "DK", "EE", "FI", "FR", "DE", "GR",
	"HU", "IE", "IT", "LV", "LT", "LU", "MT", "NL", "PL", "PT", "RO", "SK",
	"SI", "ES", "SE", "IS", "LI", "NO", "GB", "CH",
]);

function handleGeo(request: Request): Response {
	const cf = (request as Request & { cf?: { country?: string } }).cf;
	const country = (request.headers.get("CF-IPCountry") ?? cf?.country ?? "").toUpperCase();
	const consentRequired = country === "" || CONSENT_REQUIRED_COUNTRIES.has(country);
	return new Response(JSON.stringify({ country, consentRequired }), {
		status: 200,
		headers: {
			"Content-Type": "application/json",
			"Cache-Control": "no-store",
		},
	});
}
