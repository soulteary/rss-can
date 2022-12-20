const config = {
    URL: "https://www.3dmgame.com/gl_47_1/",
    // optional
    // Mode: "ssr",
    // Expire: 10*60,
    DisableCache: true,

    Title: ".bt",
    Author: "3DM Game",
    Category: "流程攻略",
    DateTime: ".item span.time",
    Description: "p",
    Link: "a",

    Pager: ".pagination li a",
    PagerLimit: 3,
  };
  potted.SetConfig(config).SetScope(".content.news .list li");
  