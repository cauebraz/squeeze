'use strict';

const http = require('node:http');
const fetch = require('node-fetch');
const { http: redirectingHttp } = require('follow-redirects');

async function fetchFeed(url) {
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`feed request failed: ${res.status}`);
  }
  return res.json();
}

function fetchText(url) {
  return new Promise((resolve, reject) => {
    redirectingHttp
      .get(url, (res) => {
        let body = '';
        res.setEncoding('utf8');
        res.on('data', (chunk) => {
          body += chunk;
        });
        res.on('end', () => resolve({ status: res.statusCode, url: res.responseUrl, body }));
      })
      .on('error', reject);
  });
}

module.exports = { fetchFeed, fetchText, http };
