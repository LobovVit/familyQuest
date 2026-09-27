export type BuildInfo = { version: string; commit: string; builtAt: string }
export type DeploymentInfo = { server: BuildInfo & { startedAt: string }; access?: BuildInfo & { startedAt: string }; web: BuildInfo }
