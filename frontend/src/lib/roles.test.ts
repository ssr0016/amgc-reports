import { beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "./api";
import { actionWords, changeUserRole, createRole, deleteRole, groupPermissions, isBuiltIn, listPermissions, updateRole } from "./roles";

vi.mock("./api", () => ({ api: vi.fn() }));

const p = (id: number, name: string) => {
  const [resource, action] = name.split(":");
  return { id, name, resource, action };
};

describe("groupPermissions", () => {
  it("groups by category in the order Reports, Users, Admin, then the rest by name", () => {
    const groups = groupPermissions([
      p(1, "users:read"),
      p(2, "zebra:feed"),
      p(3, "roles:write"),
      p(4, "reports:approve"),
      p(5, "permissions:read"),
      p(6, "audit:read"),
      p(7, "reports:read"),
    ]);
    expect(groups.map((g) => g.label)).toEqual(["Reports", "Users", "Admin", "Audit", "Zebra"]);
    expect(groups[0].permissions.map((x) => x.name)).toEqual(["reports:approve", "reports:read"]);
    expect(groups[2].permissions.map((x) => x.name)).toEqual(["permissions:read", "roles:write"]);
  });

  it("leaves out empty categories", () => {
    expect(groupPermissions([p(1, "users:read")]).map((g) => g.label)).toEqual(["Users"]);
  });
});

describe("actionWords", () => {
  it("says a known action in words and an unknown one from its code", () => {
    expect(actionWords("read_any")).toBe("View anyone's");
    expect(actionWords("approve")).toBe("Approve");
    expect(actionWords("feed_all")).toBe("Feed all");
  });
});

it("isBuiltIn knows admin, editor and user only", () => {
  expect(["admin", "editor", "user", "Reviewer", "Admin"].map(isBuiltIn)).toEqual([true, true, true, false, false]);
});

describe("requests", () => {
  beforeEach(() => vi.mocked(api).mockReset());

  it("lists permissions from the paged response", async () => {
    vi.mocked(api).mockResolvedValueOnce({ data: [p(1, "users:read")], meta: {} });
    expect(await listPermissions()).toEqual([p(1, "users:read")]);
    expect(api).toHaveBeenCalledWith("/admin/permissions?limit=100");
  });

  it("create, update and delete send the whole role", async () => {
    const body = { name: "Reviewer", description: "", permission_ids: [1, 2] };
    await createRole(body);
    expect(api).toHaveBeenLastCalledWith("/admin/roles", { method: "POST", body });
    await updateRole(4, body);
    expect(api).toHaveBeenLastCalledWith("/admin/roles/4", { method: "PUT", body });
    await deleteRole(4);
    expect(api).toHaveBeenLastCalledWith("/admin/roles/4", { method: "DELETE" });
    await changeUserRole(7, 4);
    expect(api).toHaveBeenLastCalledWith("/admin/users/7/role", { method: "PUT", body: { role_id: 4 } });
  });
});
