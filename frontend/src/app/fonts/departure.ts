import localFont from "next/font/local";

// Departure Mono — a pixel/terminal typeface by Helena Zhang & Tobias Fried,
// bundled under the SIL Open Font License 1.1 (see ./OFL.txt). Self-hosted (not
// fetched at build) so a network-restricted CI build never breaks. Used only on
// the public status console, whose whole aesthetic is an old-school CRT terminal.
export const departureMono = localFont({
  src: "./DepartureMono-Regular.woff2",
  variable: "--font-departure",
  display: "swap",
  weight: "400",
});
