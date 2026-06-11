const mode = import.meta.env.VITE_TW_MODE || "traceway";

async function bootstrap() {
  if (mode === "honeycomb") {
    await import("./instrument-honeycomb");
  } else {
    await import("./instrument-traceway");
  }
  await import("./app");
  const modeEl = document.getElementById("mode");
  if (modeEl) {
    modeEl.textContent = `instrumentation mode: ${mode}`;
  }
}

bootstrap();
