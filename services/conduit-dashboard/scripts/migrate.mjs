// Ports the same hand-rolled migration runner every Go service in this
// project uses (see services/conduit-billing/internal/db/db.go): a
// schema_migrations tracking table, filenames applied in sorted order, each
// migration in its own transaction. Plain Node, not TypeScript — this is a
// one-off startup script, not part of the app's type-checked surface, so
// pulling in a TS runtime for it would be effort spent for no real gain.
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import pg from "pg";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const migrationsDir = path.join(__dirname, "..", "migrations");

async function main() {
  const databaseUrl = process.env.DASHBOARD_DATABASE_URL;
  if (!databaseUrl) {
    throw new Error("DASHBOARD_DATABASE_URL is required");
  }

  const pool = new pg.Pool({ connectionString: databaseUrl });
  try {
    await pool.query(`
      CREATE TABLE IF NOT EXISTS schema_migrations (
        filename   TEXT PRIMARY KEY,
        applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
      )
    `);

    const filenames = (await readdir(migrationsDir))
      .filter((f) => f.endsWith(".sql"))
      .sort();

    for (const filename of filenames) {
      const { rows } = await pool.query(
        "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)",
        [filename],
      );
      if (rows[0].exists) {
        continue;
      }

      const sql = await readFile(path.join(migrationsDir, filename), "utf8");
      const client = await pool.connect();
      try {
        await client.query("BEGIN");
        await client.query(sql);
        await client.query(
          "INSERT INTO schema_migrations (filename) VALUES ($1)",
          [filename],
        );
        await client.query("COMMIT");
        console.log(`applied ${filename}`);
      } catch (err) {
        await client.query("ROLLBACK");
        throw new Error(`applying migration ${filename}: ${err.message}`);
      } finally {
        client.release();
      }
    }
  } finally {
    await pool.end();
  }
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
