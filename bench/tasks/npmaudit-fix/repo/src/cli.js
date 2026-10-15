'use strict';

const minimist = require('minimist');
const ms = require('ms');

function parseArgs(argv) {
  const args = minimist(argv, {
    string: ['feed', 'out', 'max-age'],
    boolean: ['verbose'],
    alias: { f: 'feed', o: 'out', v: 'verbose' },
    default: { out: 'reports', 'max-age': '1d' },
  });
  if (!args.feed) {
    throw new Error('--feed is required');
  }
  return {
    feed: args.feed,
    out: args.out,
    maxAge: ms(args['max-age']),
    verbose: args.verbose,
  };
}

module.exports = { parseArgs };
