const { createServer } = require("http");
const { handler } = require("./build/handler.js");
const path = require("path");

const server = createServer((req, res) => {
  // Set proper MIME types for JavaScript modules
  const url = req.url;
  if (url.endsWith(".js")) {
    res.setHeader("Content-Type", "application/javascript");
  } else if (url.endsWith(".mjs")) {
    res.setHeader("Content-Type", "application/javascript");
  } else if (url.endsWith(".css")) {
    res.setHeader("Content-Type", "text/css");
  } else if (url.includes("/_app/immutable/") || url.includes("/_app/")) {
    const ext = path.extname(url);
    if (ext === ".js" || ext === ".mjs" || ext === "") {
      res.setHeader("Content-Type", "application/javascript");
    } else if (ext === ".css") {
      res.setHeader("Content-Type", "text/css");
    }
  }
  return handler(req, res);
});

server.listen(3000, () => {
  console.log("Listening on port 3000");
});
