/// <reference path="./worker-configuration.d.ts" />
import { Container, getContainer } from "@cloudflare/containers";

interface Env extends Cloudflare.Env {
  DATABASE_URL: string;
  TASMANE_INTERNAL_BINDING_SECRET: string;
  TASMANE_IMPACT_TOKEN_SECRET: string;
  TASMANE_GOOGLE_TOKEN_ENCRYPTION_KEY: string;
}

export class TasmaneApiContainer extends Container<Env> {
  override defaultPort = 8080;
  override sleepAfter = "10m";

  override envVars = {
    DATABASE_URL: this.env.DATABASE_URL,
    TASMANE_INTERNAL_BINDING_SECRET:
      this.env.TASMANE_INTERNAL_BINDING_SECRET,
    TASMANE_IMPACT_TOKEN_SECRET:
      this.env.TASMANE_IMPACT_TOKEN_SECRET,
    TASMANE_GOOGLE_TOKEN_ENCRYPTION_KEY:
      this.env.TASMANE_GOOGLE_TOKEN_ENCRYPTION_KEY,

    TASMANE_HTTP_ADDR: "0.0.0.0:8080",
  };
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const container = getContainer(env.API_CONTAINER, "api");

    return container.fetch(request);
  },
} satisfies ExportedHandler<Env>;