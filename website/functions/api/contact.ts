interface Env {
  SENDGRID_API_KEY: string;
  SENDGRID_FROM_EMAIL: string;
  SENDGRID_TO_EMAIL: string;
}

export const onRequestPost: PagesFunction<Env> = async (context) => {
  const { request, env } = context;

  try {
    const formData = await request.json<any>();

    if (
      !formData.subject ||
      !formData.email ||
      !formData.message ||
      !formData.customerType
    ) {
      return new Response(
        JSON.stringify({ error: "Missing required fields" }),
        {
          status: 400,
          headers: { "Content-Type": "application/json" },
        }
      );
    }

    const msg = {
      personalizations: [
        {
          to: [{ email: env.SENDGRID_TO_EMAIL }],
        },
      ],
      from: { email: env.SENDGRID_FROM_EMAIL },
      subject: `New Contact Request: ${formData.subject}`,
      content: [
        {
          type: "text/plain",
          value: `
New contact request from Traceway website:

Subject: ${formData.subject}
Email: ${formData.email}
Customer Type: ${formData.customerType}

Message:
${formData.message}
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
};
