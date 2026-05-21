import Image from "next/image";
import { LoginForm } from "@/components/shared/login-form";
import { env } from "@/lib/server/env";

// Read OIDC env at request time so the OIDC button reflects the pod's
// current env vars, not whatever was set when `next build` ran.
export const dynamic = "force-dynamic";

export default function LoginPage() {
  return (
    <div className="min-h-dvh flex items-center justify-center bg-night p-4">
      <div className="w-full max-w-sm">
        <div className="flex justify-center mb-8">
          <Image
            src="/nightshift-text.png"
            alt="Nightshift"
            width={512}
            height={96}
            priority
            className="nightshift-logo-dark h-12 w-auto"
          />
          <Image
            src="/nightshift-text-black.png"
            alt="Nightshift"
            width={512}
            height={96}
            priority
            className="nightshift-logo-light h-12 w-auto"
          />
        </div>
        <LoginForm
          oidcEnabled={env.OIDC_ENABLED}
          emailPasswordEnabled={env.AUTH_EMAIL_PASSWORD_ENABLED}
        />
      </div>
    </div>
  );
}
