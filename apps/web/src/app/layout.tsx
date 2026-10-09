import type { Metadata } from "next";
import { Instrument_Sans, Newsreader } from "next/font/google";

import { AppShell } from "@/components/layout/app-shell";
import { Providers } from "@/components/providers";
import "./globals.css";

const instrumentSans = Instrument_Sans({
  variable: "--font-instrument",
  subsets: ["latin"],
});

const newsreader = Newsreader({
  variable: "--font-newsreader",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Anki Vocabulary Manager",
  description: "Discover vocabulary and create enriched Anki cards.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className={`${instrumentSans.variable} ${newsreader.variable}`}>
      <body>
        <Providers>
          <AppShell>{children}</AppShell>
        </Providers>
      </body>
    </html>
  );
}
