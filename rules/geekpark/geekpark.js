const config = {
    URL: "https://www.geekpark.net/",
    // optional
    // Mode: "ssr",
    // Expire: 10*60,
    Title: "a > .multiline-text-overflow",
    Author: "极客公园",
    Category: ".category-tag",
    DateTime: ".article-time",
    Description: ".multiline-text-overflow",
    Link: ".img-cover-wrap",

    Content: "",
    "ContentBefore": {
      action: "readlink",
      object: "#article-body",
      URL: ".img-cover-wrap"
    },
  };
  potted.SetConfig(config).SetScope(".article-list .article-item");
  