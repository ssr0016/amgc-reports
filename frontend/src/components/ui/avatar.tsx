"use client"

import Image from "next/image"
import { useState } from "react"
import { cn } from "cn"

// Tailwind keeps only class names it can see written out, so the palette is a literal list.
const COLORS = [
  "bg-red-700", "bg-orange-700", "bg-amber-700", "bg-lime-700", "bg-green-700", "bg-emerald-700",
  "bg-teal-700", "bg-cyan-700", "bg-sky-700", "bg-blue-700", "bg-indigo-700", "bg-violet-700",
  "bg-purple-700", "bg-fuchsia-700", "bg-pink-700", "bg-rose-700", "bg-slate-600",
]

// The first letter of the first two words: "Juan Dela Cruz" is JD, "test" is T.
function initials(name: string) {
  const letters = name.trim().split(/\s+/).slice(0, 2).map((word) => [...word][0] ?? "")
  return letters.join("").toLocaleUpperCase() || "?"
}

// djb2 over code points: the same name always gets the same color.
function colorOf(name: string) {
  let hash = 5381
  for (const char of name) hash = (hash * 33 + char.codePointAt(0)!) >>> 0
  return COLORS[hash % COLORS.length]
}

// Decorative: the name is always printed next to it, so screen readers skip it.
function Avatar({ name, src, className }: { name: string; src?: string; className?: string }) {
  const [failedSrc, setFailedSrc] = useState<string | null>(null)
  return (
    <span
      aria-hidden="true"
      className={cn("relative inline-flex size-8 shrink-0 items-center justify-center overflow-hidden rounded-full text-xs font-medium text-white select-none", colorOf(name), className)}
    >
      {initials(name)}
      {src && src !== failedSrc && (
        <Image
          src={src}
          alt=""
          width={512}
          height={512}
          unoptimized
          className="absolute inset-0 size-full object-cover"
          onError={() => setFailedSrc(src)}
        />
      )}
    </span>
  )
}

export { Avatar }
