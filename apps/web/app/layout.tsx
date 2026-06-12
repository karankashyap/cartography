import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { GraphQLProvider } from "@/lib/graphql/urql-provider";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Cartograph — Local-First Analytics",
  description: "AI-powered e-commerce analytics. 100% on-device.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
    >
      <body className="min-h-full flex flex-col">
        <GraphQLProvider>
          <nav className="shrink-0 border-b bg-background/95 backdrop-blur">
            <div className="mx-auto flex max-w-6xl items-center gap-6 px-6 py-3">
              <span className="font-semibold tracking-tight">Cartograph</span>
              <div className="flex gap-4 text-sm">
                <a href="/dashboard" className="text-muted-foreground hover:text-foreground transition-colors">
                  Dashboard
                </a>
                <a href="/chat" className="text-muted-foreground hover:text-foreground transition-colors">
                  Chat
                </a>
                <a href="/content" className="text-muted-foreground hover:text-foreground transition-colors">
                  Content
                </a>
              </div>
            </div>
          </nav>
          {children}
        </GraphQLProvider>
      </body>
    </html>
  );
}
