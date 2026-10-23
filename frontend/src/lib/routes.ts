// Plain constants, free of React, so proxy.ts can import them too.

// Where a signed-in user lands when there is no (safe) ?next=.
export const HOME = "/reports";

// Set by the session while signed in; proxy.ts sends visitors without it to /login.
export const SIGNED_IN_COOKIE = "signed_in";
