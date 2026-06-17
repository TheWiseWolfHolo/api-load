import { appLinks, getConfiguredReleaseRepository } from "./appLinks";

const docsUrl: string | null = appLinks.docsUrl;
const repositoryUrl: string | null = appLinks.repositoryUrl;
const releaseRepository: string | null = getConfiguredReleaseRepository();

void docsUrl;
void repositoryUrl;
void releaseRepository;
