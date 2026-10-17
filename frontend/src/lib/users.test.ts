import { afterEach, describe, expect, it, vi } from "vitest";
import { resetCsrf } from "./api";
import { changeUserStatus, createUser, listRoles, listUsers, resetUserPassword, updateUser } from "./users";

afterEach(() => vi.unstubAllGlobals());

const stub = (...bodies: unknown[]) => {
  const f = vi.fn();
  for (const b of bodies) f.mockResolvedValueOnce(new Response(JSON.stringify(b)));
  vi.stubGlobal("fetch", f);
  return f;
};
const call = (f: ReturnType<typeof vi.fn>, i = 0) => [String(f.mock.calls[i][0]).replace(/^undefined/, ""), f.mock.calls[i][1]] as const;

describe("users", () => {
  it("listUsers filters by email only when given one", async () => {
    let f = stub([]);
    await listUsers();
    expect(call(f)[0]).toBe("/users?limit=100");
    f = stub([]);
    await listUsers("a+b@x.com");
    expect(call(f)[0]).toBe("/users?limit=100&email=a%2Bb%40x.com");
  });

  it("listUsers adds the status unless it is empty or all", async () => {
    let f = stub([]);
    await listUsers("", "inactive");
    expect(call(f)[0]).toBe("/users?limit=100&status=inactive");
    f = stub([]);
    await listUsers("a@x.com", "all");
    expect(call(f)[0]).toBe("/users?limit=100&email=a%40x.com");
    f = stub([]);
    await listUsers("", "");
    expect(call(f)[0]).toBe("/users?limit=100");
  });

  it("listRoles unwraps the { data } envelope", async () => {
    stub({ data: [{ id: 1, name: "admin" }] });
    expect(await listRoles()).toEqual([{ id: 1, name: "admin" }]);
  });

  it("createUser POSTs and updateUser PUTs, with a CSRF token", async () => {
    const f = stub({ csrf_token: "t" }, { id: 5 }, { id: 5 });
    await createUser({ email: "a@b.c" } as never);
    await updateUser(5, { name: "N" } as never);
    expect(call(f, 1)[0]).toBe("/admin/users");
    expect(call(f, 1)[1]).toMatchObject({ method: "POST" });
    expect(call(f, 2)[0]).toBe("/admin/users/5");
    expect(call(f, 2)[1]).toMatchObject({ method: "PUT" });
  });

  it("changeUserStatus PUTs the status to the user's status path", async () => {
    resetCsrf(); // the token is cached across tests; start from none
    const f = stub({ csrf_token: "t" }, { id: 5, status: "inactive" });
    await changeUserStatus(5, "inactive");
    expect(call(f, 1)[0]).toBe("/admin/users/5/status");
    expect(call(f, 1)[1]).toMatchObject({ method: "PUT", body: JSON.stringify({ status: "inactive" }) });
  });

  it("resetUserPassword POSTs both passwords to the user's reset path and answers nothing", async () => {
    resetCsrf();
    const f = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ csrf_token: "t" })))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", f);
    await expect(resetUserPassword(5, "New-Passw0rd-x", "New-Passw0rd-x")).resolves.toBeUndefined();
    expect(call(f, 1)[0]).toBe("/admin/users/5/reset-password");
    expect(call(f, 1)[1]).toMatchObject({ method: "POST" });
    expect(JSON.parse(call(f, 1)[1].body)).toEqual({ new_password: "New-Passw0rd-x", confirm_password: "New-Passw0rd-x" });
  });
});
