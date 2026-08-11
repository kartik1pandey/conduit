import { Pool } from "pg";

// Next.js hot-reloads modules in dev, which would otherwise re-run this
// file and open a fresh Pool (and fresh connections) on every edit —
// stashing the pool on globalThis survives a module reload the same way
// it's a standard workaround for Prisma/pg clients in Next.js dev mode.
const globalForDb = globalThis as unknown as { dashboardDbPool?: Pool };

function createPool(): Pool {
  const databaseUrl = process.env.DASHBOARD_DATABASE_URL;
  if (!databaseUrl) {
    throw new Error("DASHBOARD_DATABASE_URL is required");
  }
  return new Pool({ connectionString: databaseUrl });
}

// Lazy by design: `next build` imports every route module to collect its
// metadata, with no runtime environment (no real DASHBOARD_DATABASE_URL) —
// a Pool constructed eagerly at module load would make every production
// build fail outside a fully-configured environment. Deferring construction
// to first actual query means the build never needs live infrastructure.
export const pool = new Proxy({} as Pool, {
  get(_target, prop) {
    const real = globalForDb.dashboardDbPool ?? createPool();
    if (process.env.NODE_ENV !== "production") {
      globalForDb.dashboardDbPool = real;
    }
    // Bind functions to the real Pool instance, not the proxy — pg's
    // Pool methods close over private instance state via `this`, which
    // would otherwise resolve to this Proxy and break at call time.
    const value = Reflect.get(real, prop, real);
    return typeof value === "function" ? value.bind(real) : value;
  },
});
