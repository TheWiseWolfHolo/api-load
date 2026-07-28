import type { AxiosRequestConfig } from "axios";
import http from "./http";

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
