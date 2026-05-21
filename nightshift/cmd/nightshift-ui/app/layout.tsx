import "@/styles/tailwind.css";
import { GeistPixelSquare } from "geist/font/pixel";
import { Instrument_Sans, JetBrains_Mono, Work_Sans } from "next/font/google";

const instrumentSans = Instrument_Sans({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-instrument-sans",
  display: "swap",
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-jetbrains-mono",
  display: "swap",
});

const workSans = Work_Sans({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-work-sans",
  display: "swap",
});

export const metadata = {
  title: "Nightshift",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html
      lang="en"
      data-theme="light"
      className={`h-full overflow-hidden ${GeistPixelSquare.variable} ${instrumentSans.variable} ${jetbrainsMono.variable} ${workSans.variable}`}
    >
      <body className="h-full overflow-hidden antialiased">{children}</body>
    </html>
  );
}
