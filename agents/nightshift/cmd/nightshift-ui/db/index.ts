import { Pool } from "pg";
import { drizzle } from "drizzle-orm/node-postgres";
import * as schema from "./schema";

let _db: ReturnType<typeof drizzle> | null = null;
let _pool: Pool | null = null;

export function getDb() {
  if (!_db) {
    const connectionString = process.env.DATABASE_URL;
    if (!connectionString) {
      throw new Error(
        "DATABASE_URL is required (Postgres URL, e.g. postgres://user:pw@host:5432/db?sslmode=disable)",
      );
    }
    // pg lazily connects on first query, so this constructor itself
    // doesn't fail against an unreachable host — important for
    // `next build`, which evaluates module-level code (including
    // better-auth's adapter init) without ever issuing a query.
    _pool = new Pool({ connectionString });
    _db = drizzle(_pool, { schema });
  }
  return _db;
}

export { schema };
export * from "drizzle-orm";
