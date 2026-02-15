export type Settings = {
  project: string;
  natsURL: string;
  httpPort: number;
  vcsBaseURL: string;
  gitUser: string;
  gitToken: string;
  name: string;
};

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`${name} is required`);
  }
  return value;
}

export const settings: Settings = {
  project: requiredEnv("PROJECT"),
  natsURL: requiredEnv("NATS_URL"),
  httpPort: Number.parseInt(process.env.HTTP_PORT ?? "8080", 10) || 8080,
  vcsBaseURL: (process.env.VCS_BASE_URL ?? "https://github.com").replace(/\/$/, ""),
  gitUser: process.env.GIT_USER ?? "",
  gitToken: process.env.GIT_TOKEN ?? "",
  name: process.env.NAME ?? "",
};

export function log(msg: string, attrs: Record<string, unknown> = {}) {
  const parts = Object.entries(attrs).map(([k, v]) => `${k}=${String(v)}`);
  if (parts.length > 0) {
    console.log(`${new Date().toISOString()} [INFO] ${msg} ${parts.join(" ")}`);
    return;
  }
  console.log(`${new Date().toISOString()} [INFO] ${msg}`);
}
