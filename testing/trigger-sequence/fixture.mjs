import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
const sdkPath =
  process.env.SEQUENCE_SDK_BUNDLE ??
  fileURLToPath(
    new URL(
      "../../../js-client/packages/frontend/dist/traceway.iife.global.js",
      import.meta.url,
    ),
  );
const server = createServer(async (req, res) => {
  if (req.url === "/sdk.js") {
    res.setHeader("Content-Type", "application/javascript");
    res.end(await readFile(sdkPath));
    return;
  }
  res.setHeader("Content-Type", "text/html");
  res.end(`<!doctype html><html><head><title>User directory · Sequence demo</title><style>body{font:17px system-ui;background:#f3f6fa;color:#142039;padding:60px;max-width:900px;margin:auto}header{font-size:14px;color:#59708c}h1{font-size:42px}button,a{display:inline-block;background:#225ce7;color:white;border:0;border-radius:8px;padding:14px 20px;margin:8px;text-decoration:none;font:inherit}#profile{padding:24px;background:white;border:1px solid #dce2ef;border-radius:16px}#hint{padding:16px;background:#edf3ff;border-radius:8px}</style></head><body><header>TRACEWAY · CONTROLLED DEMO</header><h1>User directory</h1><p>Explore a profile, navigate to its details, then confirm that it loaded.</p><div id="profile"><h2>Alex Morgan</h2><div id="hint">Hover over this card to inspect the profile.</div><button id="open-user"><span>Open user</span></button><a id="full-navigation" href="/users/42">Open in a new page</a><button id="confirm">Confirm user loaded</button><p id="status">Ready to explore</p></div><script src="/sdk.js"></script><script>
Traceway.init('shop-dev-token@http://localhost:8082/api/report',{exportDelayMs:100,captureWebVitals:false});
window.sdk=Traceway;
document.querySelector('#open-user').onclick=()=>{history.pushState({},'', '/users/42');document.querySelector('#status').textContent='User profile opened';};
document.querySelector('#confirm').onclick=()=>{console.info('User loaded');document.querySelector('#status').textContent='User details loaded successfully';};
window.ready=Traceway.refreshConfig();
</script></body></html>`);
});
server.listen(5190, "127.0.0.1", () =>
  console.log("Sequence fixture: http://localhost:5190"),
);
