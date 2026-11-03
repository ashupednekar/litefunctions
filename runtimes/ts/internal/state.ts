import { connect, type JetStreamClient, type NatsConnection } from "nats";

import { log, settings } from "./conf";

export class AppState {
  nc: NatsConnection;
  js: JetStreamClient;

  private constructor(nc: NatsConnection, js: JetStreamClient) {
    this.nc = nc;
    this.js = js;
  }

  static async new(): Promise<AppState> {
    const nc = await connect({ servers: settings.natsURL });
    const jsm = await nc.jetstreamManager();

    try {
      await jsm.streams.add({
        name: settings.project,
        subjects: [`${settings.project}.>`],
      });
    } catch (err) {
      const text = String(err).toLowerCase();
      if (!text.includes("name already in use") && !text.includes("stream name already in use")) {
        throw err;
      }
    }

    log("settings", {
      project: settings.project,
      nats_url: settings.natsURL,
      http_port: settings.httpPort,
      vcs_base_url: settings.vcsBaseURL,
      git_user: settings.gitUser,
    });

    return new AppState(nc, nc.jetstream());
  }
}
