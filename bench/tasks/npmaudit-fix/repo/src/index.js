'use strict';

const { parseArgs } = require('./cli');
const { fetchFeed } = require('./feed');
const { summarize, requester, writeReport } = require('./report');

async function main(argv, env) {
  const opts = parseArgs(argv);
  const items = await fetchFeed(opts.feed);
  const summary = summarize(items, Date.now());
  const { file } = writeReport(opts.out, summary, requester(env.HTTP_COOKIE));
  if (opts.verbose) {
    console.log(`wrote ${file}`);
  }
  return file;
}

if (require.main === module) {
  main(process.argv.slice(2), process.env).catch((err) => {
    console.error(err.message);
    process.exitCode = 1;
  });
}

module.exports = { main };
