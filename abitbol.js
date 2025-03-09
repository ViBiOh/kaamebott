#!/usr/bin/env node

import { parse } from "node-html-parser";
import he from "he";

const baseURL = "http://george-abitbol.fr";

async function crawl() {
  let output = [];

  let response = await fetch(baseURL);
  if (!response.ok) {
    throw new Error(`Response status: ${response.status}`);
  }

  let payload = await response.text();
  const index = parse(payload);

  for (let elem of index.querySelectorAll(".play")) {
    let href = elem.getAttribute("href");
    let response = await fetch(baseURL + "/" + href);
    if (!response.ok) {
      throw new Error(`Response status for ${href}: ${response.status}`);
    }

    let payload = await response.text();
    const page = parse(payload);

    output.push({
      id: href.replace("v/", ""),
      context: he
        .decode(
          page
            .querySelector('meta[property="og:title"]')
            .getAttribute("content"),
        )
        .replace(" | George Abitbol.fr", ""),
      value: he.decode(page.querySelector("blockquote > p").innerText),
      url: page
        .querySelector('meta[property="og:url"]')
        .getAttribute("content"),
      image: page
        .querySelector('meta[property="og:image"]')
        .getAttribute("content"),
    });
  }

  console.log(JSON.stringify(output));
}

crawl();
