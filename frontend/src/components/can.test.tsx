import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Can } from "./can";

const useSession = vi.hoisted(() => vi.fn());
vi.mock("@/lib/session", () => ({ useSession }));

const grant = (...perms: string[]) => useSession.mockReturnValue({ can: (p: string) => perms.includes(p) });

describe("<Can>", () => {
  it("shows its children when the user has the permission", () => {
    grant("reports:approve");
    render(<Can permission="reports:approve">Approve</Can>);
    expect(screen.getByText("Approve")).toBeInTheDocument();
  });

  it("shows nothing without it, or the fallback when given one", () => {
    grant("reports:read");
    const { container } = render(<Can permission="reports:approve">Approve</Can>);
    expect(container).toBeEmptyDOMElement();
    render(
      <Can permission="reports:approve" fallback="Not allowed">
        Approve
      </Can>,
    );
    expect(screen.getByText("Not allowed")).toBeInTheDocument();
    expect(screen.queryByText("Approve")).not.toBeInTheDocument();
  });
});
