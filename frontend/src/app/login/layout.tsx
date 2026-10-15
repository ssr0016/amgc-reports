export const metadata = { title: "Sign in" };

// The login page is a Client Component, which cannot export metadata, so its title lives here.
export default function LoginLayout({ children }: { children: React.ReactNode }) {
  return children;
}
