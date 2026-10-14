// These pages are Client Components, which cannot export `instant`, so this
// Server Component layout opts the route out of instant-navigation validation.
// The form loads after navigation by design; the page shows its own loading state.
export const instant = false;

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
