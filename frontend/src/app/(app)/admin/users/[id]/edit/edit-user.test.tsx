import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { EditUser } from "./edit-user";

const h = vi.hoisted(() => ({
  me: { user_id: 1 },
  status: "active",
  changeUserStatus: vi.fn(),
  toast: { success: vi.fn(), error: vi.fn() },
  replace: vi.fn(),
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "7" }),
  useRouter: () => ({ replace: h.replace, push: vi.fn() }),
}));
vi.mock("sonner", () => ({ toast: h.toast }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ isAdmin: true, user: h.me }) }));
vi.mock("@/lib/users", () => ({
  getUser: vi.fn(async () => ({ id: 7, name: "Ana Cruz", email: "ana@x.com", status: h.status, area_assignment: "", church_name: "" })),
  updateUser: vi.fn(),
  changeUserStatus: h.changeUserStatus,
  resetUserPassword: vi.fn(),
}));

function renderEdit() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <EditUser />
    </QueryClientProvider>,
  );
}
const section = async () => within(await screen.findByRole("group", { name: "Status" }));

beforeEach(() => {
  vi.clearAllMocks();
  h.me = { user_id: 1 };
  h.status = "active";
  h.changeUserStatus.mockResolvedValue({ id: 7, status: "inactive" });
});

describe("Edit user: status", () => {
  it("has a labelled select on the current status, and Update status is off until it changes", async () => {
    renderEdit();
    const s = await section();
    const select = s.getByLabelText("Status") as HTMLSelectElement;
    expect(select.value).toBe("active");
    expect([...select.options].map((o) => o.text)).toEqual(["Active", "Inactive", "Disciplinary", "Deceased"]);
    expect(s.getByRole("button", { name: "Update status" })).toBeDisabled();
    fireEvent.change(select, { target: { value: "inactive" } });
    expect(s.getByRole("button", { name: "Update status" })).toBeEnabled();
  });

  it("asks before moving a user away from Active, and Cancel changes nothing", async () => {
    renderEdit();
    const s = await section();
    fireEvent.change(s.getByLabelText("Status"), { target: { value: "inactive" } });
    fireEvent.click(s.getByRole("button", { name: "Update status" }));

    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent("Ana Cruz will be signed out everywhere");
    expect(dialog).toHaveTextContent("until the status is set back to Active");
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
    expect(h.changeUserStatus).not.toHaveBeenCalled();
  });

  it("confirming calls the API once and announces the result", async () => {
    renderEdit();
    const s = await section();
    fireEvent.change(s.getByLabelText("Status"), { target: { value: "inactive" } });
    fireEvent.click(s.getByRole("button", { name: "Update status" }));
    fireEvent.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Change status" }));

    await waitFor(() => expect(h.changeUserStatus).toHaveBeenCalledTimes(1));
    expect(h.changeUserStatus).toHaveBeenCalledWith(7, "inactive");
    await waitFor(() => expect(h.toast.success).toHaveBeenCalledWith("Ana Cruz is now Inactive"));
  });

  it("moving back to Active needs no confirmation", async () => {
    h.status = "deceased";
    renderEdit();
    const s = await section();
    fireEvent.change(s.getByLabelText("Status"), { target: { value: "active" } });
    fireEvent.click(s.getByRole("button", { name: "Update status" }));

    await waitFor(() => expect(h.changeUserStatus).toHaveBeenCalledWith(7, "active"));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("shows the server's message in an alert and keeps the choice", async () => {
    h.changeUserStatus.mockRejectedValueOnce(new ApiError(409, "CONFLICT", "At least one active admin is required."));
    renderEdit();
    const s = await section();
    fireEvent.change(s.getByLabelText("Status"), { target: { value: "disciplinary" } });
    fireEvent.click(s.getByRole("button", { name: "Update status" }));
    fireEvent.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Change status" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("At least one active admin is required.");
    expect((s.getByLabelText("Status") as HTMLSelectElement).value).toBe("disciplinary");
  });

  it("on your own account the select is disabled with a note", async () => {
    h.me = { user_id: 7 };
    renderEdit();
    const s = await section();
    const select = s.getByLabelText("Status");
    expect(select).toBeDisabled();
    expect(select).toHaveAccessibleDescription("You cannot change your own status.");
    expect(s.getByRole("button", { name: "Update status" })).toBeDisabled();
  });
});

describe("Edit user: reset password", () => {
  it("shows a Reset password section under Status, off on your own account", async () => {
    renderEdit();
    const reset = await screen.findByRole("group", { name: "Reset password" });
    const status = screen.getByRole("group", { name: "Status" });
    expect(status.compareDocumentPosition(reset) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(within(reset).getByRole("button", { name: "Reset password" })).toBeEnabled();
  });

  it("disables it on the signed-in admin's own page", async () => {
    h.me = { user_id: 7 };
    renderEdit();
    const reset = await screen.findByRole("group", { name: "Reset password" });
    expect(within(reset).getByRole("button", { name: "Reset password" })).toBeDisabled();
  });
});
