'use strict';

const fs = require('node:fs');
const path = require('node:path');
const _ = require('lodash');
const ms = require('ms');
const mkdirp = require('mkdirp');
const { nanoid } = require('nanoid');
const cookie = require('cookie');

function summarize(items, now) {
  const byWarehouse = _.groupBy(items, 'warehouse');
  return _.sortBy(
    Object.keys(byWarehouse).map((warehouse) => {
      const rows = byWarehouse[warehouse];
      const oldest = _.minBy(rows, 'updatedAt').updatedAt;
      return {
        warehouse,
        skus: _.uniq(rows.map((r) => r.sku)).length,
        units: _.sumBy(rows, 'units'),
        staleFor: ms(now - oldest),
      };
    }),
    'warehouse',
  );
}

function requester(cookieHeader) {
  const parsed = cookie.parse(cookieHeader || '');
  return parsed.user || 'anonymous';
}

function writeReport(dir, summary, user) {
  mkdirp.sync(dir);
  const id = nanoid(10);
  const file = path.join(dir, `report-${id}.json`);
  fs.writeFileSync(file, JSON.stringify({ id, user, summary }, null, 2) + '\n');
  return { id, file };
}

module.exports = { summarize, requester, writeReport };
