import { defineConfig } from "drizzle-kit";

export default defineConfig({
  dialect: "postgresql",
  schema: "./db/schema/index.ts",
  out: "./db/drizzle",
  dbCredentials: {
    // Drizzle-kit needs a DSN it can parse. Production migrations
    // are run by the chart's Helm Job with the real DATABASE_URL
    // injected via Secret; locally devs override this env var.
    url: process.env.DATABASE_URL || "postgres://placeholder@localhost:5432/placeholder",
  },
});
