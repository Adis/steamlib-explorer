export function ErrorBox({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      role="alert"
      className={`rounded border border-amber-700 bg-amber-950 px-4 py-3 text-sm text-amber-200 ${className}`}
    >
      {children}
    </div>
  );
}
