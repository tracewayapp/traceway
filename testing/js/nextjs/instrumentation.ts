export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    const traceway = await import("@traceway/backend");
    const endpoint =
      process.env.TRACEWAY_ENDPOINT ||
      "default_token_change_me@http://localhost:8082/api/report";
    traceway.init(endpoint, { debug: true });
  }
}
