const trimTrailingSlash = (value: string): string => value.replace(/\/+$/, "");

function optionalEnvUrl(value: string | undefined): string | null {
  const trimmed = value?.trim();
  return trimmed ? trimTrailingSlash(trimmed) : null;
}

export const appLinks = {
  repositoryUrl: optionalEnvUrl(import.meta.env.VITE_REPOSITORY_URL),
  docsUrl: optionalEnvUrl(import.meta.env.VITE_DOCS_URL),
};

export function getConfiguredReleaseRepository(): string | null {
  const explicitRepo = import.meta.env.VITE_RELEASE_REPOSITORY?.trim();
  if (explicitRepo) {
    return explicitRepo;
  }

  if (!appLinks.repositoryUrl) {
    return null;
  }

  const match = appLinks.repositoryUrl.match(/^https:\/\/github\.com\/([^/]+\/[^/]+)$/);
  return match?.[1] ?? null;
}
