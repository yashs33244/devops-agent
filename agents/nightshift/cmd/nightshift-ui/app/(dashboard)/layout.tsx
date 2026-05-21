import { requireTenantAuth } from "@/lib/server/tenant";
import { redirect } from "next/navigation";
import { DashboardShell } from "../sidebar";

export default async function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const auth = await requireTenantAuth();
  if (!auth) redirect("/login");

  return (
    <DashboardShell
      user={{
        name: auth.session.user.name,
        email: auth.session.user.email,
        orgName: auth.org.name,
        role: auth.session.user.role,
      }}
    >
      {children}
    </DashboardShell>
  );
}
