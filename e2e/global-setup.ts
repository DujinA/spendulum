// Waits until the app answers /healthz, because the distroless image has no
// shell for a compose healthcheck.
export default async function globalSetup() {
  const healthz = new URL("/healthz", process.env.BASE_URL ?? "http://localhost:8080");
  const deadline = Date.now() + 60_000;

  while (Date.now() < deadline) {
    try {
      const res = await fetch(healthz);
      if (res.ok) return;
    } catch {
      // Not listening yet.
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }

  throw new Error(`${healthz.href} did not answer 200 within 60 s`);
}
