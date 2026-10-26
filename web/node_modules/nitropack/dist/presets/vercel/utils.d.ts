import type { Nitro, PrerenderRoute } from "nitropack/types";
export declare function generateFunctionFiles(nitro: Nitro): Promise<void>;
export declare function generateEdgeFunctionFiles(nitro: Nitro): Promise<void>;
export declare function generateStaticFiles(nitro: Nitro): Promise<void>;
/**
 * Map prerendered files to the route and content type they should be served with.
 *
 * Paths are always slash-free: Vercel strips slashes when matching, so a path
 * that keeps a trailing slash matches nothing at all (#4392), and the root
 * route has to map to an empty path (ufo's slash helpers cannot produce one).
 *
 * Files that Vercel already serves at the route using its built-in directory
 * indexes keep their path, and only get an entry when their content type
 * cannot be inferred from the file name.
 */
export declare function getPrerenderOverrides(prerenderedRoutes?: PrerenderRoute[]): Record<string, {
    path?: string;
    contentType?: string;
}>;
export declare function deprecateSWR(nitro: Nitro): void;
export interface VercelConfig {
    bunVersion?: string;
}
export declare function readVercelConfig(rootDir: string): Promise<VercelConfig>;
