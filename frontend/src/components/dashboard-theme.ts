import { Inter, JetBrains_Mono } from "next/font/google";
import "./reports-dashboard.css";
import "./account-screens.css";

const inter = Inter({ subsets: ["latin"], display: "swap" });
const mono = JetBrains_Mono({ subsets: ["latin"], variable: "--reports-mono", display: "swap" });

export const dashboardTheme = `reports-dashboard ${inter.className} ${mono.variable}`;
