const config = {
  Name: "智源社区",
  URL: "https://hub.baai.ac.cn/",
  // The first response leaves the list empty and stores stories in window.__NUXT__.
  IdByRegexp: "\\/(\\d+)$",

  Title: ".story-item-title",
  Author: ".story-item-author-name",
  Category: "最新",
  DateTime: ".story-item-time",
  Description: ".story-item-summary",
  Link: ".story-item > a",
};
potted.SetConfig(config).SetScope("main .story-list .story-list-container");

// The visible timestamp includes an action label, not part of the date.
const getData = potted.GetData.bind(potted);
potted.GetData = function (...args) {
  getData(...args);
  const items = JSON.parse(this.value);
  for (const item of items) {
    if (item.date) item.date = item.date.replace(/\s+分享$/, "").replace(" ", "T") + ":00+08:00";
  }
  this.value = JSON.stringify(items);
  return this;
};
