import type { AxiosRequestConfig } from "axios";
import http from "./http";

export function createExportFilename(
  parts: Array<string | number | null | undefined>,
  extension: string,
  now = new Date()
): string {
  const safeParts = parts
    .map(part => sanitizeFilenamePart(part))
    .filter((part): part is string => Boolean(part));
  const timestamp = [
    now.getFullYear(),
    padDatePart(now.getMonth() + 1),
    padDatePart(now.getDate()),
  ].join("");
  const time = [
    padDatePart(now.getHours()),
    padDatePart(now.getMinutes()),
    padDatePart(now.getSeconds()),
  ].join("");
  const safeExtension = extension.replace(/[^a-z0-9]/gi, "").toLowerCase() || "txt";
  return `${safeParts.join("-")}-${timestamp}-${time}.${safeExtension}`;
}

function sanitizeFilenamePart(part: string | number | null | undefined): string {
  if (part === null || part === undefined) {
    return "";
  }
  return String(part)
    .normalize("NFKC")
    .trim()
    .replace(/[<>:"/\\|?*]|\p{Cc}/gu, "-")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^[.\-\s]+|[.\-\s]+$/g, "")
    .slice(0, 64);
}

function padDatePart(value: number): string {
  return String(value).padStart(2, "0");
}

export function downloadAuthenticatedFile(
  path: string,
  filename: string,
  params?: AxiosRequestConfig["params"]
): void {
  void http
    .get<Blob, Blob>(path, {
      params,
      responseType: "blob",
      hideMessage: true,
    })
    .then(blob => {
      const objectUrl = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = objectUrl;
      link.download = filename;
      link.style.display = "none";
      document.body.appendChild(link);

      try {
        link.click();
      } finally {
        link.remove();
        window.setTimeout(() => URL.revokeObjectURL(objectUrl), 0);
      }
    })
    .catch(() => {
      // The shared HTTP interceptor already reports authentication and request errors.
    });
}
