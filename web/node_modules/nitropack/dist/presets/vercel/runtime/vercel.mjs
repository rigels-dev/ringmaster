import "#nitro-internal-pollyfills";
import { useNitroApp } from "nitropack/runtime";
import { getRouteRulesForPath } from "nitropack/runtime/internal/index";
import { toNodeListener } from "h3";
import { parseQuery, withQuery } from "ufo";
import { ISR_URL_PARAM } from "./consts.mjs";
const nitroApp = useNitroApp();
const handler = toNodeListener(nitroApp.h3App);
const listener = function(req, res) {
  const queryIndex = req.url.indexOf("?");
  const { [ISR_URL_PARAM]: queryURL, ...params } = queryIndex === -1 ? {} : parseQuery(req.url.slice(queryIndex));
  const xNowRouteMatches = req.headers["x-now-route-matches"];
  const headerURL = xNowRouteMatches ? parseQuery(xNowRouteMatches)[ISR_URL_PARAM] : void 0;
  const url = headerURL && typeof headerURL === "string" ? headerURL : queryURL;
  if (url && typeof url === "string") {
    const routeRules = getRouteRulesForPath(url);
    if (routeRules.isr) {
      req.url = Object.keys(params).length > 0 ? withQuery(url, params) : url;
    }
  }
  return handler(req, res);
};
export default listener;
