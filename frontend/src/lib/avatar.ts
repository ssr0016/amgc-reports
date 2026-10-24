import { api } from "./api";
import type { User } from "./api-schema";

export async function resizeAvatar(file: File): Promise<Blob> {
  if (!["image/jpeg", "image/png", "image/webp"].includes(file.type)) {
    throw new Error("Choose a JPEG, PNG or WebP image.");
  }
  const url = URL.createObjectURL(file);
  try {
    const image = new Image();
    image.src = url;
    await image.decode().catch(() => { throw new Error("This image could not be opened. Choose another file."); });
    const scale = Math.min(1, 512 / Math.max(image.naturalWidth, image.naturalHeight));
    const canvas = document.createElement("canvas");
    canvas.width = Math.max(1, Math.round(image.naturalWidth * scale));
    canvas.height = Math.max(1, Math.round(image.naturalHeight * scale));
    const context = canvas.getContext("2d");
    if (!context) throw new Error("Your browser could not resize this image.");
    context.fillStyle = "#ffffff";
    context.fillRect(0, 0, canvas.width, canvas.height);
    context.drawImage(image, 0, 0, canvas.width, canvas.height);
    const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/jpeg", 0.85));
    if (!blob) throw new Error("Your browser could not resize this image.");
    // Leave room for the multipart boundary and headers under the API's 1 MiB cap.
    if (blob.size > 900 * 1024) throw new Error("This image is still too large. Choose another file.");
    return blob;
  } finally {
    URL.revokeObjectURL(url);
  }
}

export function uploadAvatar(blob: Blob): Promise<User> {
  const body = new FormData();
  body.append("avatar", blob, "avatar.jpg");
  return api<User>("/users/me/avatar", { method: "POST", body });
}
