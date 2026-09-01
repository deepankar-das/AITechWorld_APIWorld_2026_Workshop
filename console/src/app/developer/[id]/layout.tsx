/**
 * Author: Deepankar Das
 */

export function generateStaticParams() {
  // Pre-generate "me" route for Sentinel Console.
  // Other IDs are handled at runtime via client-side navigation.
  return [{ id: "me" }];
}

export default function DeveloperLayout({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}
