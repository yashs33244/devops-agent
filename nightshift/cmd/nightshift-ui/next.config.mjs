/** @type {import('next').NextConfig} */
export default {
  serverExternalPackages: ["better-auth", "better-sqlite3"],
  devIndicators: false,
  // Surface CR0N_MASCOT to client code at build time (without requiring the
  // NEXT_PUBLIC_ prefix). Empty string when unset → mascot is hidden. Valid
  // values: "clawd", "gemini", "codex".
  env: {
    CR0N_MASCOT: process.env.CR0N_MASCOT ?? "",
  },
};
