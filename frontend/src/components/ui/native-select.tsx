import { cn } from "cn"

// A plain <select> styled like Input; one dropdown does not need Radix Select.
// The input background is translucent but the popup is drawn by the browser on
// its own (light) panel, so options get a solid theme color or they read as
// light text on white in dark mode.
function NativeSelect({ className, ...props }: React.ComponentProps<"select">) {
  return (
    <select
      data-slot="native-select"
      className={cn(
        "[&_option]:bg-background [&_option]:text-foreground h-8 w-full rounded-lg border border-input bg-transparent px-2.5 text-base outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 md:text-sm dark:bg-input/30",
        className
      )}
      {...props}
    />
  )
}

export { NativeSelect }
