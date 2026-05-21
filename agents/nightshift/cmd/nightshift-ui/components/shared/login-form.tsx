"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { authClient } from "@/auth/client";
import { cn } from "@/lib/utils";

export function LoginForm({
  className,
  redirectTo = "/tasks",
  oidcEnabled = false,
  emailPasswordEnabled = false,
  ...props
}: React.ComponentProps<"div"> & {
  redirectTo?: string;
  oidcEnabled?: boolean;
  emailPasswordEnabled?: boolean;
}) {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [mode, setMode] = useState<"signin" | "signup">("signin");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);

    const { error: err } = mode === "signup"
      ? await authClient.signUp.email({ email, password, name })
      : await authClient.signIn.email({ email, password });

    setLoading(false);
    if (err) { setError(err.message || `Sign ${mode === "signup" ? "up" : "in"} failed`); return; }
    router.push(redirectTo);
  }

  async function handleOidcSignIn() {
    setError("");
    setLoading(true);
    const { error: err } = await authClient.signIn.oauth2({
      providerId: "openbao",
      callbackURL: redirectTo,
    });
    if (err) {
      setLoading(false);
      setError(err.message || "OIDC sign-in failed");
    }
    // Successful flow redirects the browser — no need to clear loading.
  }

  return (
    <div className={cn("flex flex-col gap-6", className)} {...props}>
      <form
        onSubmit={handleSubmit}
        className="rounded-2xl border border-lime/10 bg-lime/[0.02] backdrop-blur-xl shadow-lg p-6 space-y-5"
      >
        <div className="text-center mb-2">
          <h1 className="text-xl font-semibold text-primary">
            Welcome
          </h1>
          <p className="text-sm text-muted mt-1">
            {mode === "signin"
              ? "Sign in to your account"
              : "Enter your details to get started"}
          </p>
        </div>

        {emailPasswordEnabled && mode === "signup" && (
          <div className="space-y-1.5">
            <label htmlFor="name" className="block text-sm font-medium text-secondary">
              Name
            </label>
            <input
              id="name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Jane Doe"
              required
              autoComplete="name"
              className="form-input text-base"
            />
          </div>
        )}

        {emailPasswordEnabled && (
          <div className="space-y-1.5">
            <label htmlFor="email" className="block text-sm font-medium text-secondary">
              Email
            </label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@company.com"
              required
              autoComplete="email"
              className="form-input text-base"
            />
          </div>
        )}

        {emailPasswordEnabled && (
          <div className="space-y-1.5">
            <label htmlFor="password" className="block text-sm font-medium text-secondary">
              Password
            </label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Enter your password"
              required
              autoComplete={mode === "signup" ? "new-password" : "current-password"}
              className="form-input text-base"
            />
          </div>
        )}

        {error && (
          <div className="rounded-lg border border-error/30 bg-error/10 px-4 py-2 text-sm text-error">
            {error}
          </div>
        )}

        {emailPasswordEnabled && (
          <button
            type="submit"
            disabled={loading}
            className="w-full rounded-lg bg-lime py-3 text-base font-semibold text-night hover:bg-lime-bright focus-ring disabled:opacity-50"
          >
            {loading
              ? mode === "signin"
                ? "Signing in..."
                : "Signing up..."
              : mode === "signin"
                ? "Sign In"
                : "Sign Up"}
          </button>
        )}

        {oidcEnabled && (
          <>
            {emailPasswordEnabled && (
              <div className="relative text-center text-xs text-muted">
                <span className="bg-night/0 px-2 relative z-10">or</span>
                <span className="absolute left-0 top-1/2 w-full border-t border-lime/10" aria-hidden />
              </div>
            )}
            <button
              type="button"
              onClick={handleOidcSignIn}
              disabled={loading}
              className="w-full rounded-lg bg-lime py-3 text-base font-semibold text-night hover:bg-lime-bright focus-ring disabled:opacity-50"
            >
              {loading ? "Signing in..." : "Sign in with OpenBao"}
            </button>
          </>
        )}

        {emailPasswordEnabled && (
          <p className="text-center text-sm text-muted">
          {mode === "signin" ? (
            <>
              Don&apos;t have an account?{" "}
              <button
                type="button"
                onClick={() => { setMode("signup"); setError(""); }}
                className="text-lime hover:text-lime-bright underline underline-offset-4"
              >
                Sign up
              </button>
            </>
          ) : (
            <>
              Already have an account?{" "}
              <button
                type="button"
                onClick={() => { setMode("signin"); setError(""); }}
                className="text-lime hover:text-lime-bright underline underline-offset-4"
              >
                Sign in
              </button>
            </>
          )}
        </p>
        )}
      </form>
    </div>
  );
}
