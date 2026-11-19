interface Env {
	SENDGRID_API_KEY: string;
	SENDGRID_FROM_EMAIL: string;
	SENDGRID_TO_EMAIL: string;
}

export default {
	async fetch(request: Request, env: Env): Promise<Response> {
		const url = new URL(request.url);

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
