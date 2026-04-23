/**
 * Downloads vendored JS libraries into static/dist/.
 * Run once: node scripts/vendor.js
 */

const https = require("https");
const fs = require("fs");
const path = require("path");

const dist = path.join(__dirname, "..", "static", "dist");
fs.mkdirSync(dist, { recursive: true });

const files = [
  {
    url: "https://unpkg.com/htmx.org@2.0.3/dist/htmx.min.js",
    dest: path.join(dist, "htmx.min.js"),
  },
  {
    url: "https://unpkg.com/alpinejs@3.14.3/dist/cdn.min.js",
    dest: path.join(dist, "alpine.min.js"),
  },
];

for (const { url, dest } of files) {
  process.stdout.write(`Downloading ${path.basename(dest)}… `);
  const file = fs.createWriteStream(dest);
  https.get(url, (res) => {
    // Follow one redirect.
    if (res.statusCode === 301 || res.statusCode === 302) {
      https.get(res.headers.location, (res2) => {
        res2.pipe(file);
        file.on("finish", () => { file.close(); console.log("done"); });
      });
    } else {
      res.pipe(file);
      file.on("finish", () => { file.close(); console.log("done"); });
    }
  }).on("error", (e) => console.error(e));
}
