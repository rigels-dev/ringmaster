import "#nitro-internal-pollyfills";
import { useNitroApp } from "nitropack/runtime";
import { startScheduleRunner } from "nitropack/runtime/internal";
import wsAdapter from "crossws/adapters/bun";
const nitroApp = useNitroApp();
const ws = import.meta._websocket ? wsAdapter(nitroApp.h3App.websocket) : void 0;
function parseInt(value) {
  const parsedValue = Number.parseInt(value);
  return Number.isNaN(parsedValue) ? void 0 : parsedValue;
}
const server = Bun.serve({
  port: process.env.NITRO_PORT || process.env.PORT || 3e3,
  hostname: process.env.NITRO_HOST || process.env.HOST,
  idleTimeout: parseInt(process.env.NITRO_BUN_IDLE_TIMEOUT),
  websocket: import.meta._websocket ? ws.websocket : void 0,
  async fetch(req, server2) {
    if (import.meta._websocket && req.headers.get("upgrade") === "websocket") {
      return ws.handleUpgrade(req, server2);
    }
    const url = new URL(req.url);
    let body;
    if (req.body) {
      body = await req.arrayBuffer();
    }
    return nitroApp.localFetch(url.pathname + url.search, {
      host: url.hostname,
      protocol: url.protocol,
      headers: req.headers,
      method: req.method,
      redirect: req.redirect,
      body
    });
  }
});
console.log(`Listening on ${server.url}...`);
if (import.meta._tasks) {
  startScheduleRunner();
}
