// Compatibility shim — chunk-19 ports cr0n's UI verbatim. Every
// app/api/*/route.ts imports `{ cronProxy }` from this path; we keep
// the import working by re-exporting the nightshift-proxy.ts adapters
// under the old names. The URL/body translation happens transparently
// inside nightshiftProxy / nightshiftBinaryProxy / nightshiftSSEProxy.
//
// Future cleanup: rewrite the route.ts handlers to import from
// "@/lib/server/nightshift-proxy" directly + delete this shim. Skipped
// for chunk 19 to keep the diff minimal.

export {
  nightshiftProxy as cronProxy,
  nightshiftBinaryProxy as cronBinaryProxy,
  nightshiftSSEProxy as cronSSEProxy,
} from "./nightshift-proxy";
