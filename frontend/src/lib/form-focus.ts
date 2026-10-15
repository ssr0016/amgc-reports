import { useEffect, useRef, useState } from "react";

// What a failed submit should move focus to: the first invalid control, or, when the
// invalid field cannot take focus (a locked, disabled select), its error message.
export function focusFirstError(form: HTMLElement | null) {
  const target =
    form?.querySelector<HTMLElement>('[aria-invalid="true"]:not(:disabled):not([readonly])') ??
    form?.querySelector<HTMLElement>("[data-field-error]");
  target?.focus();
}

// Put `ref` on the <form> and call the returned function after setting errors.
// Focus moves in an effect, once the errors are in the DOM. With nothing invalid
// (a 429, say) nothing moves: the form-level alert announces it.
export function useFocusFirstError<T extends HTMLElement>() {
  const ref = useRef<T>(null);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    if (attempt > 0) focusFirstError(ref.current);
  }, [attempt]);
  return [ref, () => setAttempt((n) => n + 1)] as const;
}
