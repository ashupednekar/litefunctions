import { watchTSHook, reconcileConsumers } from "./pkg/consumer";
import { settings } from "./pkg/conf";
import { ensureRuntimeDir, invokeModule, syncRepoAndReload } from "./pkg/functions";
import { AppState } from "./pkg/state";

async function handleSyncRequest(state: AppState, req: Request): Promise<Response> {
  const name = req.headers.get("X-Litefunction-Name") || settings.name;
  if (!name) {
    return new Response("function name not provided", { status: 400 });
  }

  const body = new Uint8Array(await req.arrayBuffer());

  try {
    let first: Uint8Array | undefined;
    for await (const out of invokeModule(state, name, "", body, req)) {
      first = out;
      break;
    }
    return new Response(first ?? new Uint8Array(), {
      status: 200,
      headers: {
        "content-type": "application/json",
      },
    });
  } catch (err) {
    if (String(err).includes("not found")) {
      return new Response(`function '${name}' not found`, { status: 404 });
    }
    console.error(`${new Date().toISOString()} [ERROR] sync invoke failed`, {
      project: settings.project,
      function: name,
      error: String(err),
    });
    return new Response(String(err), { status: 500 });
  }
}

async function main(): Promise<void> {
  await ensureRuntimeDir();

  const state = await AppState.new();
  await syncRepoAndReload();
  await reconcileConsumers(state);
  await watchTSHook(state);

  Bun.serve({
    port: settings.httpPort,
    fetch: (req) => handleSyncRequest(state, req),
  });

  console.log(`${new Date().toISOString()} [INFO] bun runtime listening project=${settings.project} port=${settings.httpPort}`);
}

main().catch((err) => {
  console.error(`${new Date().toISOString()} [FATAL] runtime boot failed`, err);
  process.exit(1);
});
