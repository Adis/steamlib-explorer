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
      className={`rounded-md border border-warn-line bg-warn-bg px-4 py-3 text-sm text-warn-fg ${className}`}
    >
      {children}
    </div>
  );
}
