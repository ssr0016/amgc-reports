# Premium UX and Reliability Improvements

## Intent

Make the frontend communicate loading, mutation, navigation, empty, and deleted-record states clearly while preventing stale report edits from mutating deleted data.

## Goals

- Treat a deleted report as a terminal edit-page state with a clear recovery link.
- Keep save/submit mutations safe when a report disappears between load and click.
- Give users immediate feedback for authentication and mutations.
- Replace generic data-fetch loading text with layout-shaped skeletons in the application’s data views.
- Provide route and background-fetch activity indicators.
- Give empty data views an actionable, accessible explanation.

## Non-goals

- No new runtime dependency.
- No polling; focus refetching plus mutation-time 404 handling is sufficient.
- No broad visual rebrand or redesign of existing screens.
- No API or database changes.

## Approach

Use existing Next.js 16, React Query, Tailwind, Radix primitives, and Lucide icons. Add small local `Skeleton`, empty-state, and global activity components. Use Next’s `useLinkStatus` for link navigation progress and `useIsFetching` for background data activity. Keep mutation ownership in existing components and add loading feedback at the existing button boundaries.

## Constraints

- Preserve existing access-control and error behavior.
- Preserve dirty-form data when a save fails.
- Keep interactive controls at existing accessible target sizes and expose busy state to assistive technology.
- Do not commit secrets or modify generated Next.js agent instructions.

## Success criteria

- Focused tests cover deleted reports, skeletons, empty states, global indicators, and login/mutation busy states.
- `npm test`, `npm run typecheck`, `npm run lint`, and `npm run build` pass from `frontend/`.
