/**
 * Downloads vendored JS libraries into static/dist/.
 * Run via `make vendor` (also runs as part of `make assets`).
 * Skips files that already exist — pass --force to re-download.
 */

const https = require("https");
const fs = require("fs");
const path = require("path");

const force = process.argv.includes("--force");
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

function download(url, dest, redirectsLeft = 3) {
  return new Promise((resolve, reject) => {
    https
      .get(url, (res) => {
        if ([301, 302, 307, 308].includes(res.statusCode) && res.headers.location) {
          if (redirectsLeft <= 0) {
            reject(new Error(`too many redirects fetching ${url}`));
            return;
          }
          res.resume();
          download(res.headers.location, dest, redirectsLeft - 1).then(resolve, reject);
          return;
        }
        if (res.statusCode !== 200) {
          reject(new Error(`${url} → HTTP ${res.statusCode}`));
          return;
        }
        const file = fs.createWriteStream(dest);
        res.pipe(file);
        file.on("finish", () => file.close(resolve));
        file.on("error", reject);
      })
      .on("error", reject);
  });
}

async function main() {
  for (const { url, dest } of files) {
    const name = path.basename(dest);
    if (!force && fs.existsSync(dest) && fs.statSync(dest).size > 0) {
      console.log(`${name}: already present, skipping (use --force to re-download)`);
      continue;
    }
    process.stdout.write(`${name}: downloading from ${url} … `);
    try {
      await download(url, dest);
      const size = fs.statSync(dest).size;
      if (size === 0) throw new Error("downloaded file is empty");
      console.log(`done (${size} bytes)`);
    } catch (err) {
      console.log("FAILED");
      fs.rmSync(dest, { force: true });
      throw err;
    }
  }
}

main().catch((err) => {
  console.error(`\nvendor.js: ${err.message}`);
  process.exit(1);
});
