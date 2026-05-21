function required(key: string): string {
  const value = process.env[key];
  if (!value) throw new Error(`Missing required env var: ${key}`);
  return value;
}

export const env = {
  get DATABASE_URL() { return process.env.DATABASE_URL || "nightshift-ui.db"; },
  get BETTER_AUTH_SECRET() { return required("BETTER_AUTH_SECRET"); },
  get BETTER_AUTH_URL() { return required("BETTER_AUTH_URL"); },
  // NIGHTSHIFT_API_URL is the in-cluster URL of nightshift-api's HTTP
  // gateway (default :8080). Every cr0n-shape REST call the UI makes
  // is translated to /v1/* by lib/server/nightshift-proxy.ts before it
  // reaches this URL.
  get NIGHTSHIFT_API_URL() { return required("NIGHTSHIFT_API_URL"); },
  get TENANT_ID() { return required("TENANT_ID"); },
  get TENANT_NAME() { return required("TENANT_NAME"); },
  get OIDC_ENABLED() { return !!process.env.OIDC_CLIENT_ID && !!process.env.OIDC_CLIENT_SECRET; },
  get AUTH_EMAIL_PASSWORD_ENABLED() { return process.env.AUTH_EMAIL_PASSWORD_ENABLED === "true"; },
  // In-cluster OpenBao Service URL — used by the userpass-bridge Route
  // Handler to call /v1/auth/userpass/login + /v1/identity/oidc/...
  // server-side. Never exposed to the browser.
  get OPENBAO_INTERNAL_URL() { return required("OPENBAO_INTERNAL_URL"); },
};
