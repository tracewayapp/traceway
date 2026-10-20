// Next.js compiles this hook for both the Node.js and Edge runtimes, so it
// only imports the Node-only SDK module when it runs on Node.
export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    await import("./instrumentation.node");
  }
}
