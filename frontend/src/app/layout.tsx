import type { Metadata } from "next";
import { Fira_Code, Fira_Sans } from "next/font/google";
import "./globals.css";
import { Providers } from "./providers";

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
  title: "Beacon — Infrastructure Monitoring",
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

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    // suppressHydrationWarning: the script below mutates <html> before React
    // hydrates, which is exactly the class/style mismatch React would warn about.
    <html lang="en" className={`${firaSans.variable} ${firaCode.variable}`} suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: noFlashTheme }} />
      </head>
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
