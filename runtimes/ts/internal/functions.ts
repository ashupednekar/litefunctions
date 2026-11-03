import { mkdir, readdir, readFile, rm, stat, writeFile } from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

import { log, settings } from "./conf";
import type { AppState } from "./state";

type LoadedModule = {
  handle?: (req: RequestLike) => unknown;
};

type RequestLike = {
  json: () => Promise<unknown>;
  text: () => Promise<string>;
  bytes: () => Promise<Uint8Array>;
  arrayBuffer: () => Promise<ArrayBuffer>;
};

const modules = new Map<string, LoadedModule>();
let moduleVersion = 1;

function repoPath(): string {
  return path.join("/tmp", settings.project);
}

function repoFunctionsPath(): string {
  return path.join(repoPath(), "functions", "ts");
}

function runtimeFunctionsPath(): string {
  return path.join(import.meta.dir, "functions");
}

function encodeGitPart(value: string): string {
  return encodeURIComponent(value).replace(/%40/g, "@");
}

function repoURL(): string {
  const owner = settings.gitUser || "git";
  const projectPath = `${owner}/${settings.project}`;

  if (!settings.gitToken) {
    return `${settings.vcsBaseURL}/${projectPath}`;
  }

  const base = new URL(settings.vcsBaseURL);
  const user = encodeGitPart(owner);
  const token = encodeGitPart(settings.gitToken);
  return `${base.protocol}//${user}:${token}@${base.host}/${projectPath}`;
}

async function fileExists(p: string): Promise<boolean> {
  try {
    await stat(p);
    return true;
  } catch {
    return false;
  }
}

async function run(cmd: string[]): Promise<void> {
  const proc = Bun.spawn(cmd, {
    stdout: "pipe",
    stderr: "pipe",
  });
  const exited = await proc.exited;
  if (exited === 0) {
    return;
  }

  const out = await new Response(proc.stdout).text();
  const err = await new Response(proc.stderr).text();
  throw new Error(`command failed (${cmd.join(" ")})\nstdout: ${out}\nstderr: ${err}`);
}

async function syncRepo(): Promise<void> {
  const localPath = repoPath();
  if (await fileExists(localPath)) {
    await run(["git", "-C", localPath, "fetch", "--all", "--prune"]);

    try {
      await run(["git", "-C", localPath, "reset", "--hard", "origin/main"]);
      return;
    } catch {
      await run(["git", "-C", localPath, "reset", "--hard", "origin/master"]);
      return;
    }
  }

  await run(["git", "clone", repoURL(), localPath]);
}

function isFunctionFile(name: string): boolean {
  if (!/^[A-Za-z0-9_-]+\.ts$/.test(name)) {
    return false;
  }
  if (name.endsWith(".d.ts")) {
    return false;
  }
  return true;
}

export async function ensureRuntimeDir(): Promise<void> {
  await mkdir(runtimeFunctionsPath(), { recursive: true });
}

function clearModuleCache() {
  modules.clear();
  moduleVersion += 1;
}

async function syncRuntimeFunctionsDir(sourceRoot: string): Promise<void> {
  const runtimeRoot = runtimeFunctionsPath();
  await mkdir(runtimeRoot, { recursive: true });

  for (const entry of await readdir(runtimeRoot)) {
    if (!isFunctionFile(entry)) {
      continue;
    }
    await rm(path.join(runtimeRoot, entry), { force: true });
  }

  let copied = 0;
  if (await fileExists(sourceRoot)) {
    for (const entry of await readdir(sourceRoot)) {
      if (!isFunctionFile(entry)) {
        continue;
      }
      await writeFile(path.join(runtimeRoot, entry), await readFile(path.join(sourceRoot, entry)));
      copied += 1;
    }
  }

  clearModuleCache();

  log("repo sync complete", {
    project: settings.project,
    repo_functions_path: sourceRoot,
    runtime_functions_path: runtimeRoot,
    files_copied: copied,
  });
}

export async function syncRepoAndReload(): Promise<void> {
  log("syncing repo", {
    project: settings.project,
    repo_url: `${settings.vcsBaseURL}/${settings.gitUser}/${settings.project}`,
  });
  await syncRepo();
  await syncRuntimeFunctionsDir(repoFunctionsPath());
}

export async function listFunctions(): Promise<string[]> {
  const root = runtimeFunctionsPath();
  if (!(await fileExists(root))) {
    return [];
  }

  const names = new Set<string>();
  for (const entry of await readdir(root)) {
    if (!isFunctionFile(entry)) {
      continue;
    }
    names.add(path.basename(entry, ".ts"));
  }
  return Array.from(names).sort();
}

async function resolveFunctionFile(name: string): Promise<string> {
  const p = path.join(runtimeFunctionsPath(), `${name}.ts`);
  if (await fileExists(p)) {
    return p;
  }
  throw new Error(`function '${name}' not found in ${runtimeFunctionsPath()}`);
}

async function loadModule(name: string): Promise<LoadedModule> {
  const cached = modules.get(name);
  if (cached) {
    return cached;
  }

  const filePath = await resolveFunctionFile(name);
  const fileURL = pathToFileURL(filePath).href;
  const mod = (await import(`${fileURL}?v=${moduleVersion}`)) as LoadedModule;
  modules.set(name, mod);
  return mod;
}

async function toRequestLike(payload: Uint8Array): Promise<RequestLike> {
  return {
    json: async () => {
      if (payload.length === 0) {
        return {};
      }
      return JSON.parse(new TextDecoder().decode(payload));
    },
    text: async () => new TextDecoder().decode(payload),
    bytes: async () => payload,
    arrayBuffer: async () => payload.buffer.slice(payload.byteOffset, payload.byteOffset + payload.byteLength),
  };
}

function asBytes(value: unknown): Uint8Array {
  if (value == null) {
    return new Uint8Array();
  }
  if (value instanceof Uint8Array) {
    return value;
  }
  if (value instanceof ArrayBuffer) {
    return new Uint8Array(value);
  }
  if (value instanceof Response) {
    return new TextEncoder().encode("response objects are only supported in sync mode");
  }
  if (typeof value === "string") {
    return new TextEncoder().encode(value);
  }
  return new TextEncoder().encode(JSON.stringify(value));
}

export async function* invokeModule(
  _state: AppState,
  name: string,
  _reqID: string,
  payload: Uint8Array,
  request?: Request,
): AsyncGenerator<Uint8Array, void, void> {
  const module = await loadModule(name);
  if (typeof module.handle !== "function") {
    throw new Error(`module '${name}' does not export handle(req)`);
  }

  const reqLike = request ?? (await toRequestLike(payload));
  const result = await module.handle(reqLike);

  if (result == null) {
    yield new Uint8Array();
    return;
  }

  if (result instanceof Response) {
    const body = await result.arrayBuffer();
    yield new Uint8Array(body);
    return;
  }

  if (typeof (result as { [Symbol.asyncIterator]?: unknown })[Symbol.asyncIterator] === "function") {
    for await (const item of result as AsyncIterable<unknown>) {
      yield asBytes(item);
    }
    return;
  }

  if (typeof (result as { [Symbol.iterator]?: unknown })[Symbol.iterator] === "function" && typeof result !== "string") {
    for (const item of result as Iterable<unknown>) {
      yield asBytes(item);
    }
    return;
  }

  yield asBytes(result);
}
