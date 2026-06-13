import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { GraphQLProvider } from "@/lib/graphql/urql-provider";
import { ActiveStoreProvider } from "@/lib/active-store";
import { NavBar } from "@/components/NavBar";
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
      className={`dark ${geistSans.variable} ${geistMono.variable} h-full antialiased`}
    >
      <body className="min-h-full flex flex-col">
        <GraphQLProvider>
          <ActiveStoreProvider>
            <NavBar />
            {children}
          </ActiveStoreProvider>
        </GraphQLProvider>
      </body>
    </html>
  );
}
