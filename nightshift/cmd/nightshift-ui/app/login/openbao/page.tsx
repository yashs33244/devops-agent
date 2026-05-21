import Image from "next/image";
import { redirect } from "next/navigation";
import { UserpassForm } from "./userpass-form";

export const dynamic = "force-dynamic";

const REQUIRED_PARAMS = ["state", "redirect_uri", "client_id"] as const;

export default async function OpenBaoLoginPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const sp = await searchParams;

  // Bounce back to the login page if better-auth didn't drive us here
  // with the OAuth params it issues — `state` and `redirect_uri` are
  // the load-bearing ones for the bridge to OpenBao.
  for (const key of REQUIRED_PARAMS) {
    if (typeof sp[key] !== "string" || !(sp[key] as string)) {
      redirect("/login");
    }
  }

  // Strip the error param out of the OAuth pass-through; surface it
  // separately to the form. Without this the error would round-trip
  // back to OpenBao on the next submit (and OpenBao would reject the
  // unknown query param).
  const oauthParams: Record<string, string> = {};
  let errorMessage: string | null = null;
  for (const [k, v] of Object.entries(sp)) {
    if (typeof v !== "string") continue;
    if (k === "error") errorMessage = v;
    else oauthParams[k] = v;
  }

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
        <UserpassForm oauthParams={oauthParams} errorMessage={errorMessage} />
      </div>
    </div>
  );
}
