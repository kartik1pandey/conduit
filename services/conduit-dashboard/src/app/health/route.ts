import { pool } from "@/lib/db";

export async function GET() {
  try {
    await pool.query("SELECT 1");
  } catch {
    return Response.json({ status: "unavailable" }, { status: 503 });
  }
  return Response.json({ status: "ok" });
}
