import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { resetCsrf } from "@/lib/api";
import { SessionProvider, useSession } from "@/lib/session";
import { ProfileMenu } from "../../profile-menu";
import { ProfilePicture } from "./profile-picture";

vi.mock("@/lib/avatar", async (importOriginal) => ({
  ...await importOriginal<typeof import("@/lib/avatar")>(),
  resizeAvatar: async () => new Blob(["jpeg"], { type: "image/jpeg" }),
}));

afterEach(() => {
  vi.unstubAllGlobals();
  resetCsrf();
});

function SessionAvatar() {
  return <output aria-label="Session avatar">{useSession().user?.avatar_url}</output>;
}

it("updates the session, profile picture, and menu immediately after uploading, and retains the URL on remount", async () => {
  const avatarUrl = "https://pub-example.r2.dev/FKDOHMN2PACMYCOMAATRRV3HGR";
  const me = {
    user_id: 7, name: "Ana", email: "ana@example.com", role: "user",
    permissions: [], area_assignment: "", church_name: "", avatar_url: "",
  };
  vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit) => {
    const path = String(url).replace(/^undefined/, "");
    if (path.endsWith("/auth/me")) return Response.json(me);
    if (path.endsWith("/auth/csrf")) return Response.json({ csrf_token: "token" });
    if (path.endsWith("/users/me/avatar") && init.method === "POST") {
      expect(init.body).toBeInstanceOf(FormData);
      me.avatar_url = avatarUrl;
      return Response.json({
        id: 7, name: me.name, email: me.email, avatar_url: avatarUrl,
        role_id: 1, email_verified: true, created_at: "2026-01-01T00:00:00Z", status: "active",
      });
    }
    throw new Error(`Unexpected request: ${path}`);
  }));
  const app = (
    <SessionProvider>
      <ProfileMenu onSignOut={vi.fn()} />
      <ProfilePicture />
      <SessionAvatar />
    </SessionProvider>
  );
  const first = render(app);
  await screen.findByRole("button", { name: "Ana · user" });
  await userEvent.upload(screen.getByLabelText("Choose picture"), new File(["png"], "photo.png", { type: "image/png" }));
  await userEvent.click(screen.getByRole("button", { name: "Save picture" }));
  await waitFor(() => expect(screen.getByLabelText("Session avatar")).toHaveTextContent(avatarUrl));
  expect(first.container.querySelectorAll("img")).toHaveLength(2);
  for (const image of first.container.querySelectorAll("img")) {
    expect(image).toHaveAttribute("src", avatarUrl);
  }

  first.unmount();
  const refreshed = render(app);
  await waitFor(() => expect(screen.getByLabelText("Session avatar")).toHaveTextContent(avatarUrl));
  expect(refreshed.container.querySelectorAll("img")).toHaveLength(2);
  for (const image of refreshed.container.querySelectorAll("img")) {
    expect(image).toHaveAttribute("src", avatarUrl);
  }
});
