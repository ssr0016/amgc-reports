import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { ProfilePicture } from "./profile-picture";

const h = vi.hoisted(() => ({ resize: vi.fn(), upload: vi.fn(), update: vi.fn(), success: vi.fn(), invalidate: vi.fn() }));
vi.mock("@/lib/avatar", () => ({ resizeAvatar: h.resize, uploadAvatar: h.upload }));
vi.mock("@/lib/session", () => ({ useSession: () => ({ user: { user_id: 1, name: "Ana", avatar_url: "" }, updateAvatar: h.update }) }));
vi.mock("@/lib/query", () => ({ queryClient: { invalidateQueries: h.invalidate } }));
vi.mock("sonner", () => ({ toast: { success: h.success } }));

beforeEach(() => { vi.clearAllMocks(); });
const photo = () => new File(["image"], "photo.png", { type: "image/png" });

it("requires a file without sending a request", async () => {
  render(<ProfilePicture />);
  await userEvent.click(screen.getByRole("button", { name: "Save picture" }));
  expect(screen.getByRole("alert")).toHaveTextContent("Choose an image first");
  expect(h.upload).not.toHaveBeenCalled();
  expect(screen.getByLabelText("Choose picture")).toHaveFocus();
});

it("resizes before upload, disables the form, and updates the session avatar", async () => {
  let finish!: (value: { id: number; avatar_url: string }) => void;
  const blob = new Blob(["small"], { type: "image/jpeg" });
  h.resize.mockResolvedValue(blob);
  h.upload.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
  render(<ProfilePicture />);
  const input = screen.getByLabelText("Choose picture");
  const file = photo();
  await userEvent.upload(input, file);
  await userEvent.click(screen.getByRole("button", { name: "Save picture" }));
  expect(h.resize).toHaveBeenCalledWith(file);
  expect(h.upload).toHaveBeenCalledWith(blob);
  expect(input).toBeDisabled();
  expect(screen.getByRole("button", { name: "Uploading…" })).toBeDisabled();
  finish({ id: 1, avatar_url: "https://cdn.example/a?v=2" });
  await waitFor(() => expect(h.success).toHaveBeenCalledWith("Profile picture updated"));
  expect(h.update).toHaveBeenCalledWith(1, "https://cdn.example/a?v=2");
  expect(h.invalidate).toHaveBeenCalledWith({ queryKey: ["users"] });
});

it.each([
  [new ApiError(413, "LARGE", "large"), "This image is too large"],
  [new ApiError(422, "VALIDATION", "validation failed", { avatar: "must be an image" }), "must be an image"],
  [new ApiError(503, "UNAVAILABLE", "Uploads unavailable"), "Uploads unavailable"],
  [new Error("This image could not be opened"), "This image could not be opened"],
])("shows upload or conversion errors and allows retry", async (error, message) => {
  h.resize.mockResolvedValue(new Blob(["jpeg"]));
  h.upload.mockRejectedValue(error);
  render(<ProfilePicture />);
  await userEvent.upload(screen.getByLabelText("Choose picture"), photo());
  await userEvent.click(screen.getByRole("button", { name: "Save picture" }));
  await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(message));
  expect(h.update).not.toHaveBeenCalled();
  expect(screen.getByRole("button", { name: "Save picture" })).toBeEnabled();
});

it("does not upload when image conversion fails", async () => {
  h.resize.mockRejectedValue(new Error("This image could not be opened"));
  render(<ProfilePicture />);
  await userEvent.upload(screen.getByLabelText("Choose picture"), photo());
  await userEvent.click(screen.getByRole("button", { name: "Save picture" }));
  await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("could not be opened"));
  expect(h.upload).not.toHaveBeenCalled();
});
