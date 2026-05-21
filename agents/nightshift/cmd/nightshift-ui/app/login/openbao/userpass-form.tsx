export function UserpassForm({
  oauthParams,
  errorMessage,
}: {
  oauthParams: Record<string, string>;
  errorMessage: string | null;
}) {
  return (
    <form
      action="/api/auth/openbao/userpass"
      method="POST"
      className="rounded-2xl border border-lime/10 bg-lime/[0.02] backdrop-blur-xl shadow-lg p-6 space-y-5"
    >
      <div className="text-center mb-2">
        <h1 className="text-xl font-semibold text-primary">Sign in with OpenBao</h1>
        <p className="text-sm text-muted mt-1">
          Use your OpenBao userpass credentials.
        </p>
      </div>

      <div className="space-y-1.5">
        <label htmlFor="username" className="block text-sm font-medium text-secondary">
          Username
        </label>
        <input
          id="username"
          name="username"
          type="text"
          required
          autoComplete="username"
          className="form-input text-base"
        />
      </div>

      <div className="space-y-1.5">
        <label htmlFor="password" className="block text-sm font-medium text-secondary">
          Password
        </label>
        <input
          id="password"
          name="password"
          type="password"
          required
          autoComplete="current-password"
          className="form-input text-base"
        />
      </div>

      {Object.entries(oauthParams).map(([k, v]) => (
        <input key={k} type="hidden" name={k} value={v} />
      ))}

      {errorMessage && (
        <div className="rounded-lg border border-error/30 bg-error/10 px-4 py-2 text-sm text-error">
          {errorMessage}
        </div>
      )}

      <button
        type="submit"
        className="w-full rounded-lg bg-lime py-3 text-base font-semibold text-night hover:bg-lime-bright focus-ring"
      >
        Sign In
      </button>
    </form>
  );
}
