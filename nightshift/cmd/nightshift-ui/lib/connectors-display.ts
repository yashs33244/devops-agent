// Display metadata for the /connectors page. The catalog (server-side
// YAML) is the source of truth for *what* a connector is; this file is
// the source of truth for *how it looks*.
//
// Default icon: https://api.iconify.design/logos/<name>.svg
// (gilbarbara/logos via Iconify — full-color brand marks). Most
// providers' connector name matches the logos slug exactly and need
// no entry below.
//
// ICON_OVERRIDES values:
//   - bare slug → https://api.iconify.design/logos/<slug>.svg
//   - full URL  → used verbatim (e.g. simpleicons fallback)
//   - "/..."    → same-origin SVG vendored under public/
//
// DEVELOPER_OVERRIDES: only when the label isn't the title-cased name
// (e.g. Square → Block).
//
// TITLE_OVERRIDES: only when the catalog name doesn't title-case
// nicely (e.g. google_gmail → "Gmail").

const ICON_OVERRIDES: Record<string, string> = {
  motherduck: "/connectors/motherduck.png",
  monday: "monday-icon",
  outlook: "microsoft-outlook",
  onedrive: "microsoft-onedrive",
  sharepoint: "microsoft-sharepoint",
  "ms-teams": "microsoft-teams",
  google_gmail: "google-gmail",
  google_drive: "google-drive",
  google_calendar: "google-calendar",
  google_chat: "https://cdn.simpleicons.org/googlechat",
  google_contacts: "/connectors/google_contacts.svg",
  docusign: "/connectors/docusign.svg",
  zoho_crm: "https://cdn.simpleicons.org/zoho",
  plaid: "https://cdn.simpleicons.org/plaid",
};

const DEVELOPER_OVERRIDES: Record<string, string> = {
  square: "Block",
  wordpress: "Automattic",
  motherduck: "MotherDuck",
  zoho_crm: "Zoho",
  "ms-teams": "Microsoft",
  outlook: "Microsoft",
  onedrive: "Microsoft",
  sharepoint: "Microsoft",
  google_gmail: "Google",
  google_drive: "Google",
  google_calendar: "Google",
  google_chat: "Google",
  google_contacts: "Google",
};

const TITLE_OVERRIDES: Record<string, string> = {
  motherduck: "MotherDuck",
  google_gmail: "Gmail",
  google_drive: "Google Drive",
  google_calendar: "Google Calendar",
  google_chat: "Google Chat",
  google_contacts: "Google Contacts",
  hubspot: "HubSpot",
  monday: "monday.com",
};

export function iconUrlFor(name: string): string {
  if (!name) return "";
  const override = ICON_OVERRIDES[name] ?? name;
  if (override.startsWith("http") || override.startsWith("/")) return override;
  return `https://api.iconify.design/logos/${override}.svg`;
}

function titleCase(name: string): string {
  return name
    .split(/[-_\s]+/)
    .map((part) => (part.length > 0 ? part[0]!.toUpperCase() + part.slice(1) : ""))
    .join(" ");
}

export function titleFor(name: string): string {
  if (!name) return "";
  return TITLE_OVERRIDES[name] ?? titleCase(name);
}

export function developerFor(name: string): string {
  if (!name) return "";
  return DEVELOPER_OVERRIDES[name] ?? titleCase(name);
}
