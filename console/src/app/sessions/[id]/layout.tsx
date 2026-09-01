/**
 * Author: Deepankar Das
 */

export function generateStaticParams() {
  // Pre-generate a placeholder route for static export.
  // Actual session IDs are handled at runtime via client-side navigation.
  return [{ id: "_" }];
}

export default function SessionLayout({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}
