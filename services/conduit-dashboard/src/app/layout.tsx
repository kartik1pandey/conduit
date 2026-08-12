import type { Metadata } from "next";
import { Space_Grotesk, Inter, Geist_Mono } from "next/font/google";
import { Toaster } from "sonner";
import { MotionConfig } from "framer-motion";
import "./globals.css";

const display = Space_Grotesk({
  variable: "--font-display",
  subsets: ["latin"],
  weight: ["500", "600", "700"],
});

const body = Inter({
  variable: "--font-body",
  subsets: ["latin"],
});

const mono = Geist_Mono({
  variable: "--font-mono",
  subsets: ["latin"],
});

const SITE_URL = "https://conduit-dashboard.onrender.com";
const DESCRIPTION =
  "Payment intents, real-time risk scoring, and a double-entry ledger, backed by reliably-delivered webhooks. Test mode — no real money ever moves.";

export const metadata: Metadata = {
  metadataBase: new URL(SITE_URL),
  title: {
    default: "Conduit — Payments infrastructure",
    template: "%s · Conduit",
  },
  description: DESCRIPTION,
  openGraph: {
    title: "Conduit — Payments infrastructure",
    description: DESCRIPTION,
    url: SITE_URL,
    siteName: "Conduit",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Conduit — Payments infrastructure",
    description: DESCRIPTION,
  },
};

export const viewport = {
  themeColor: "#0f0f17",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      className={`${display.variable} ${body.variable} ${mono.variable} h-full antialiased`}
    >
      <body className="min-h-full flex flex-col">
        {/* reducedMotion="user" makes every Framer Motion animation in
            the app respect prefers-reduced-motion automatically — one
            place to get this right instead of checking it in every
            component that animates. */}
        <MotionConfig reducedMotion="user">{children}</MotionConfig>
        <Toaster theme="system" position="top-right" richColors />
      </body>
    </html>
  );
}
