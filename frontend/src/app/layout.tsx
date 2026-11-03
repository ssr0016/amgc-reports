import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import { Toaster } from "@/components/ui/sonner";
import { GlobalActivityIndicator } from "@/components/global-activity-indicator";
import { QueryProvider } from "@/lib/query";
import { SessionProvider } from "@/lib/session";
import { ThemeProvider } from "@/components/theme-provider";
import "./globals.css";

const inter = Inter({
  variable: "--font-inter",
  subsets: ["latin"],
  display: "swap",
});

const mono = JetBrains_Mono({
  variable: "--font-jetbrains-mono",
  subsets: ["latin"],
  display: "swap",
});

export const metadata: Metadata = {
  title: { default: "AMGCReports", template: "%s · AMGCReports" },
  description: "Pastoral reports for workers and churches",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      className={`${inter.variable} ${mono.variable} dark h-full antialiased`}
      suppressHydrationWarning
    >
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: `(() => { const saved = localStorage.getItem("theme"); const theme = saved === "light" || saved === "dark" ? saved : (matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark"); document.documentElement.classList.toggle("dark", theme === "dark"); })();`,
          }}
        />
      </head>
      <body className="min-h-full flex flex-col" suppressHydrationWarning>
        <QueryProvider>
          <GlobalActivityIndicator />
          <ThemeProvider><SessionProvider>{children}</SessionProvider></ThemeProvider>
          <Toaster />
        </QueryProvider>
      </body>
    </html>
  );
}
