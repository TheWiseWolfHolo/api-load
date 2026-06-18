const trimTrailingSlash = (value: string): string => value.replace(/\/+$/, "");
const defaultRepositoryUrl = "https://github.com/TheWiseWolfHolo/api-load";

function optionalEnvUrl(value: string | undefined): string | null {
  const trimmed = value?.trim();
  return trimmed ? trimTrailingSlash(trimmed) : null;
}

export const appLinks = {
  repositoryUrl: optionalEnvUrl(import.meta.env.VITE_REPOSITORY_URL) ?? defaultRepositoryUrl,
  docsUrl: optionalEnvUrl(import.meta.env.VITE_DOCS_URL),
};

export function getConfiguredReleaseRepository(): string | null {
  const explicitRepo = import.meta.env.VITE_RELEASE_REPOSITORY?.trim();
  if (explicitRepo) {
    return explicitRepo;
  }

  return null;
}
