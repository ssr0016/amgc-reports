import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { resizeAvatar, uploadAvatar } from "./avatar";
import { resetCsrf } from "./api";

const drawImage = vi.fn();
const fillRect = vi.fn();
let width = 2048;
let height = 1024;
let decode: () => Promise<void>;
let encoded: Blob | null;

beforeEach(() => {
  width = 2048;
  height = 1024;
  decode = () => Promise.resolve();
  encoded = new Blob(["jpeg"], { type: "image/jpeg" });
  vi.stubGlobal("Image", class {
    naturalWidth = width;
    naturalHeight = height;
    decode() { return decode(); }
  });
  vi.stubGlobal("URL", { createObjectURL: vi.fn(() => "blob:photo"), revokeObjectURL: vi.fn() });
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockImplementation(function (this: HTMLCanvasElement) {
    return { drawImage, fillRect } as unknown as CanvasRenderingContext2D;
  });
  vi.spyOn(HTMLCanvasElement.prototype, "toBlob").mockImplementation((cb) => cb(encoded));
  drawImage.mockClear();
  resetCsrf();
});
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

const photo = () => new File(["original"], "photo.png", { type: "image/png" });

describe("resizeAvatar", () => {
  it.each([[2048, 1024, 512, 256], [1024, 2048, 256, 512], [80, 60, 80, 60]])(
    "resizes %i × %i to %i × %i without upscaling",
    async (w, h, expectedW, expectedH) => {
      width = w; height = h;
      expect(await resizeAvatar(photo())).toBe(encoded);
      const canvas = vi.mocked(HTMLCanvasElement.prototype.getContext).mock.instances[0] as HTMLCanvasElement;
      expect([canvas?.width, canvas?.height]).toEqual([expectedW, expectedH]);
      expect(drawImage).toHaveBeenCalledWith(expect.anything(), 0, 0, expectedW, expectedH);
      expect(fillRect).toHaveBeenCalledWith(0, 0, expectedW, expectedH);
      expect(HTMLCanvasElement.prototype.toBlob).toHaveBeenCalledWith(expect.any(Function), "image/jpeg", 0.85);
      expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:photo");
    },
  );
  it("rejects unsupported file types before decoding", async () => {
    await expect(resizeAvatar(new File(["svg"], "image.svg", { type: "image/svg+xml" }))).rejects.toThrow("JPEG, PNG or WebP");
    expect(URL.createObjectURL).not.toHaveBeenCalled();
  });
  it("releases the URL when decoding fails", async () => {
    decode = () => Promise.reject(new Error("decode"));
    await expect(resizeAvatar(photo())).rejects.toThrow("could not be opened");
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:photo");
  });
  it("handles canvas encoding failure", async () => {
    encoded = null;
    await expect(resizeAvatar(photo())).rejects.toThrow("could not resize");
    expect(URL.revokeObjectURL).toHaveBeenCalled();
  });
  it("keeps headroom for multipart overhead", async () => {
    encoded = new Blob([new Uint8Array(900 * 1024 + 1)], { type: "image/jpeg" });
    await expect(resizeAvatar(photo())).rejects.toThrow("still too large");
  });
});

it("uploads the resized blob as multipart with credentials and CSRF", async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(new Response(JSON.stringify({ csrf_token: "token" })))
    .mockResolvedValueOnce(new Response(JSON.stringify({ id: 1, avatar_url: "https://cdn.example/avatar" })));
  vi.stubGlobal("fetch", fetchMock);
  const blob = new Blob(["jpeg"], { type: "image/jpeg" });
  await uploadAvatar(blob);
  const [url, init] = fetchMock.mock.calls[1];
  expect(url).toMatch(/\/users\/me\/avatar$/);
  expect(init.credentials).toBe("include");
  expect(init.headers).toEqual({ "X-CSRF-Token": "token" });
  const file = init.body.get("avatar") as File;
  expect(file.name).toBe("avatar.jpg");
  expect(file.type).toBe("image/jpeg");
  expect(file.size).toBe(blob.size);
});
