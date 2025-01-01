const config = {
  Name: "哔哩哔哩",
  URL: "https://www.bilibili.com/read/home",
  Mode: "mix",
  Title: ".article-item__title",
  Author: ".article-item__author .article-item__uname",
  Category: ".article-item__label",
  // DateTime: "(sub page)#app .title-container .article-read-info .publish-text",
  Description: ".article-item__desc",
  Link: "a.article-item",
  // IdByProp: {
  //   object: "",
  //   prop: "data-id",
  // },
};
potted.SetConfig(config).SetScope(".feed .feed-item")
