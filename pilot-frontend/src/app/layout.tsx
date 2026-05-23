import type { Metadata } from "next";
import { Inter_Tight, Instrument_Serif, JetBrains_Mono } from "next/font/google";
import "./globals.css";

const interTight = Inter_Tight({
  subsets: ["latin"],
  variable: "--font-inter-tight",
  weight: ["400", "500", "600", "700"],
});

const instrumentSerif = Instrument_Serif({
  subsets: ["latin"],
  variable: "--font-instrument-serif",
  weight: "400",
  style: ["normal", "italic"],
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-jetbrains-mono",
  weight: ["400", "500"],
});

const siteUrl = "https://devops-agent.vercel.app";

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: {
    default: "Pilot — AI DevOps Agent",
    template: "%s — Pilot",
  },
  description:
    "Pilot is an autonomous AI DevOps agent. Give it a GitHub repo and a cloud — it writes the Dockerfile, Terraform, Helm chart, and CI/CD pipeline, then monitors and scales your service automatically.",
  keywords: [
    "devops agent",
    "AI devops",
    "autonomous devops",
    "devops automation",
    "terraform generator",
    "helm chart generator",
    "github actions ci cd",
    "kubernetes deployment",
    "docker automation",
    "infrastructure as code AI",
    "claude code devops",
    "sre automation",
    "keda scale to zero",
    "aws eks terraform",
    "gcp gke autopilot",
  ],
  authors: [{ name: "Pilot", url: siteUrl }],
  creator: "Pilot",
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
      "max-video-preview": -1,
      "max-image-preview": "large",
      "max-snippet": -1,
    },
  },
  openGraph: {
    type: "website",
    locale: "en_US",
    url: siteUrl,
    siteName: "Pilot",
    title: "Pilot — AI DevOps Agent",
    description:
      "Autonomous DevOps agent. Give it a repo — it writes the Dockerfile, Terraform, Helm chart, and CI/CD pipeline automatically.",
    images: [{ url: "/og-image.png", width: 1200, height: 630, alt: "Pilot — AI DevOps Agent" }],
  },
  twitter: {
    card: "summary_large_image",
    title: "Pilot — AI DevOps Agent",
    description:
      "Autonomous DevOps agent. Give it a repo — it writes the Dockerfile, Terraform, Helm chart, and CI/CD pipeline automatically.",
    images: ["/og-image.png"],
    creator: "@yashs33244",
  },
  icons: {
    icon: [{ url: "/icon.svg", type: "image/svg+xml" }],
    apple: [{ url: "/apple-icon.svg", type: "image/svg+xml" }],
    shortcut: "/icon.svg",
  },
  alternates: {
    canonical: siteUrl,
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={`${interTight.variable} ${instrumentSerif.variable} ${jetbrainsMono.variable}`}
    >
      <head>
        <meta name="theme-color" content="#0a0a0b" />
        <link rel="icon" href="/icon.svg" type="image/svg+xml" />
        <link rel="apple-touch-icon" href="/apple-icon.svg" />
      </head>
      <body suppressHydrationWarning>{children}</body>
    </html>
  );
}
