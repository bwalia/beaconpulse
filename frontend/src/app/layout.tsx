import type { Metadata } from "next";
import { Fira_Code, Fira_Sans } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getLocale } from "next-intl/server";
import "./globals.css";
import { Providers } from "./providers";
import { localeDir, resolveLocale } from "@/i18n/config";

// next/font downloads and self-hosts these at BUILD time, so the running app never
// calls fonts.googleapis.com — important for a self-hosted product — and the font
// files are preloaded with size-adjust metrics, so swapping in the real face causes
// no layout shift.
// Only the weights the app actually renders: 400 body, 500 medium, 600 semibold,
// 700 bold. (`font-light` is used nowhere.) Every extra weight is another
// preloaded file on first paint.
const firaSans = Fira_Sans({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-sans",
  display: "swap",
});

// Mono is only ever rendered at the default weight (targets, slugs, PromQL).
const firaCode = Fira_Code({
  subsets: ["latin"],
  weight: ["400"],
  variable: "--font-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: "Beacon Pulse — Infrastructure Monitoring",
  description: "Self-hosted infrastructure monitoring platform",
};

// Runs before first paint, so the page never flashes light before hydration swaps
// it to dark. Dependency-free and wrapped in try/catch because localStorage throws
// in some privacy modes. Must stay in sync with THEME_KEY and applyTheme() in
// src/lib/theme.tsx.
const noFlashTheme = `
(function(){try{
  var t = localStorage.getItem('beacon-theme');
  var dark = t === 'dark' || (t !== 'light' && matchMedia('(prefers-color-scheme: dark)').matches);
  document.documentElement.classList.toggle('dark', dark);
  document.documentElement.style.colorScheme = dark ? 'dark' : 'light';
}catch(e){}})();
`;

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  // The active locale, resolved from the cookie by i18n/request.ts. lang and dir are set
  // from it so the browser reads the page in the right language and, for Arabic, the
  // right direction. Messages are provided to the client tree once, here.
  const locale = resolveLocale(await getLocale());
  return (
    // suppressHydrationWarning: the script below mutates <html> before React
    // hydrates, which is exactly the class/style mismatch React would warn about.
    <html
      lang={locale}
      dir={localeDir(locale)}
      className={`${firaSans.variable} ${firaCode.variable}`}
      suppressHydrationWarning
    >
      <head>
        <script dangerouslySetInnerHTML={{ __html: noFlashTheme }} />
      </head>
      <body>
        {/* Messages load automatically from the request config, so no explicit prop is
            needed — the provider makes them available to every client component. */}
        <NextIntlClientProvider>
          <Providers>{children}</Providers>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
