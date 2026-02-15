import type { AppState } from "./state";
import { log, settings } from "./conf";
import { invokeModule, listFunctions, syncRepoAndReload } from "./functions";

const CONSUMERS = new Map<string, { cancel: () => void; task: Promise<void> }>();

export async function consumeFunction(state: AppState, name: string): Promise<void> {
  const subject = `${settings.project}.${name}.exec.ts.*`;
  const sub = state.nc.subscribe(subject);

  const task = (async () => {
    for await (const msg of sub) {
      const parts = msg.subject.split(".");
      const reqID = parts[4] ?? "";
      const resSubject = `${settings.project}.${name}.res.ts.${reqID}`;
      try {
        for await (const out of invokeModule(state, name, reqID, msg.data)) {
          state.nc.publish(resSubject, out);
        }
      } catch (err) {
        console.error(`${new Date().toISOString()} [ERROR] async invoke failed`, {
          project: settings.project,
          function: name,
          error: String(err),
        });
      }
    }
  })();

  CONSUMERS.set(name, { cancel: () => sub.unsubscribe(), task });
  log("ts consumer started", { function: name, subject });
}

export async function reconcileConsumers(state: AppState): Promise<void> {
  const desired = new Set(await listFunctions());

  for (const [name, consumer] of CONSUMERS.entries()) {
    if (desired.has(name)) {
      continue;
    }
    consumer.cancel();
    CONSUMERS.delete(name);
  }

  for (const name of desired) {
    if (CONSUMERS.has(name)) {
      continue;
    }
    await consumeFunction(state, name);
  }
}

export async function watchTSHook(state: AppState): Promise<void> {
  const subjects = [`${settings.project}.hook.ts`];
  log("ts hook listener started", { project: settings.project, subjects: subjects.join(",") });

  for (const subject of subjects) {
    const sub = state.nc.subscribe(subject);
    void (async () => {
      for await (const msg of sub) {
        try {
          log("ts hook received", {
            project: settings.project,
            subject: msg.subject,
            payload: new TextDecoder().decode(msg.data),
          });
          await syncRepoAndReload();
          await reconcileConsumers(state);
          log("runtime code refreshed via hook", { project: settings.project });
        } catch (err) {
          console.error(`${new Date().toISOString()} [ERROR] failed to refresh runtime code`, {
            project: settings.project,
            error: String(err),
          });
        }
      }
    })();
  }
}
